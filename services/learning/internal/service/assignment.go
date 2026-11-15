package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	identityClient "ego/api/gen/go/identity"
	"ego/platform/httpx"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"
)

// CreateAssignments giao một bài tập cho nhiều lớp cùng lúc: tạo một đề riêng (private) dùng
// chung, rồi mỗi lớp một assignment với lịch riêng (giờ mở, hạn nộp, thời lượng, giám sát).
// Mọi lớp được kiểm trước khi mở transaction, để không giao dở dang cho một nửa số lớp.
func (s *service) CreateAssignments(ctx context.Context, teacherID string, req dto.CreateAssignmentsRequest) (*dto.CreatedAssignmentsResponse, *httpx.HTTPError) {
	// Kiểm tra toàn bộ lịch, lớp trùng và quyền sở hữu trước khi mở transaction.
	seenClassroomIDs := make(map[uint]struct{}, len(req.Schedules))
	for _, schedule := range req.Schedules {
		if _, ok := seenClassroomIDs[schedule.ClassroomID]; ok {
			return nil, ErrAssignmentAlreadyExists
		}
		seenClassroomIDs[schedule.ClassroomID] = struct{}{}
		if !schedule.DueAt.After(schedule.OpensAt) {
			return nil, ErrAssignmentInvalidTime
		}
		if time.Duration(schedule.Duration)*time.Minute > schedule.DueAt.Sub(schedule.OpensAt) {
			return nil, ErrAssignmentDuration
		}
		classroom, err := s.repo.GetClassroomByID(ctx, schedule.ClassroomID)
		if err != nil {
			return nil, httpx.FromError(err, errClassroomServiceFailed)
		}
		if classroom.TeacherID != teacherID {
			return nil, ErrClassroomForbidden
		}
		if !classroom.Active {
			return nil, ErrClassroomNotActive
		}
	}

	// Chuẩn hóa chuỗi nhập trước khi tạo đề, để tiêu đề và nội dung không dính khoảng trắng thừa.
	examReq := dto.CreatePrivateExamRequest{
		Title:       strings.TrimSpace(req.Title),
		Description: req.Description,
		Questions:   make([]dto.CreatePrivateExamQuestionRequest, len(req.Questions)),
	}
	for i, question := range req.Questions {
		options := make([]string, len(question.Options))
		for optionIndex, option := range question.Options {
			options[optionIndex] = strings.TrimSpace(option)
		}
		examReq.Questions[i] = dto.CreatePrivateExamQuestionRequest{
			Content:            strings.TrimSpace(question.Content),
			Explanation:        question.Explanation,
			Options:            options,
			CorrectOptionIndex: question.CorrectOptionIndex,
			ExcludedFromScore:  question.ExcludedFromScore,
		}
	}

	var assignments []*model.Assignment
	// Đề private, mọi assignment và thông báo nằm trong một transaction: lỗi ở bất
	// kỳ bước nào thì không còn đề mồ côi, không cần xóa bù.
	if err := s.repo.Transaction(ctx, func(repo repository.Repository) error {
		exam, err := s.withRepo(repo).createPrivateExam(ctx, examReq)
		if err != nil {
			return err
		}
		assignments = make([]*model.Assignment, 0, len(req.Schedules))
		for _, schedule := range req.Schedules {
			assignments = append(assignments, &model.Assignment{
				ClassroomID:   schedule.ClassroomID,
				Title:         strings.TrimSpace(req.Title),
				Description:   req.Description,
				ExamID:        exam.ID,
				Duration:      schedule.Duration,
				OpensAt:       schedule.OpensAt,
				DueAt:         schedule.DueAt,
				Active:        true,
				Proctored:     schedule.Proctored,
				MaxViolations: maxViolationsOrDefault(schedule.MaxViolations),
			})
		}

		// Ghi toàn bộ Assignment sau khi mọi lớp đều đã vượt qua bước kiểm tra.
		if err := repo.CreateAssignments(ctx, assignments); err != nil {
			return err
		}
		for _, assignment := range assignments {
			members, err := repo.GetApprovedMembersByClassroom(ctx, assignment.ClassroomID)
			if err != nil {
				return err
			}
			recipients := make([]string, len(members))
			for i, member := range members {
				recipients[i] = member.UserID
			}
			if err := createNotificationOutbox(ctx, repo, model.NotificationSourceAssignment, fmt.Sprintf("assignment:%d:created", assignment.ID), recipients,
				"New assignment", assignment.Title, "ASSIGNMENT", assignment.ID, assignment.ClassroomID, time.Time{}); err != nil {
				return err
			}
			reminderAt := assignment.DueAt.Add(-24 * time.Hour)
			if reminderAt.Before(time.Now()) {
				reminderAt = time.Now()
			}
			if err := createNotificationOutbox(ctx, repo, model.NotificationSourceAssignment, fmt.Sprintf("assignment:%d:due-reminder", assignment.ID), recipients,
				"Assignment due soon", assignment.Title+" is due soon", "ASSIGNMENT", assignment.ID, assignment.ClassroomID, reminderAt); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}

	return dto.ToCreatedAssignmentsResponse(assignments), nil
}

// assignExistingExamTx giao đề của source cho thêm các lớp (cùng giáo viên, đang hoạt động,
// chưa có đề này) trong transaction của người gọi, kèm thông báo tạo bài và nhắc hạn.
func assignExistingExamTx(ctx context.Context, repo repository.Repository, teacherID string, source *model.Assignment, schedules []dto.AssignmentScheduleRequest) ([]*model.Assignment, error) {
	seen := make(map[uint]struct{}, len(schedules))
	for _, schedule := range schedules {
		if _, ok := seen[schedule.ClassroomID]; ok {
			return nil, ErrAssignmentAlreadyExists
		}
		seen[schedule.ClassroomID] = struct{}{}
		if !schedule.DueAt.After(schedule.OpensAt) {
			return nil, ErrAssignmentInvalidTime
		}
		if time.Duration(schedule.Duration)*time.Minute > schedule.DueAt.Sub(schedule.OpensAt) {
			return nil, ErrAssignmentDuration
		}
		classroom, err := repo.GetClassroomByID(ctx, schedule.ClassroomID)
		if err != nil {
			return nil, err
		}
		if classroom.TeacherID != teacherID {
			return nil, ErrClassroomForbidden
		}
		if !classroom.Active {
			return nil, ErrClassroomNotActive
		}
		exists, err := repo.AssignmentExistsByClassroomAndExam(ctx, schedule.ClassroomID, source.ExamID)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, ErrAssignmentAlreadyExists
		}
	}

	assignments := make([]*model.Assignment, 0, len(schedules))
	for _, schedule := range schedules {
		assignments = append(assignments, &model.Assignment{
			ClassroomID:   schedule.ClassroomID,
			Title:         source.Title,
			Description:   source.Description,
			ExamID:        source.ExamID,
			Duration:      schedule.Duration,
			OpensAt:       schedule.OpensAt,
			DueAt:         schedule.DueAt,
			Active:        true,
			Proctored:     schedule.Proctored,
			MaxViolations: maxViolationsOrDefault(schedule.MaxViolations),
		})
	}
	if err := repo.CreateAssignments(ctx, assignments); err != nil {
		return nil, err
	}
	for _, assignment := range assignments {
		members, err := repo.GetApprovedMembersByClassroom(ctx, assignment.ClassroomID)
		if err != nil {
			return nil, err
		}
		recipients := make([]string, len(members))
		for i, member := range members {
			recipients[i] = member.UserID
		}
		if err := createNotificationOutbox(ctx, repo, model.NotificationSourceAssignment, fmt.Sprintf("assignment:%d:created", assignment.ID), recipients,
			"New assignment", assignment.Title, "ASSIGNMENT", assignment.ID, assignment.ClassroomID, time.Time{}); err != nil {
			return nil, err
		}
		reminderAt := assignment.DueAt.Add(-24 * time.Hour)
		if reminderAt.Before(time.Now()) {
			reminderAt = time.Now()
		}
		if err := createNotificationOutbox(ctx, repo, model.NotificationSourceAssignment, fmt.Sprintf("assignment:%d:due-reminder", assignment.ID), recipients,
			"Assignment due soon", assignment.Title+" is due soon", "ASSIGNMENT", assignment.ID, assignment.ClassroomID, reminderAt); err != nil {
			return nil, err
		}
	}
	return assignments, nil
}

// GetAssignments liệt kê bài tập của lớp có phân trang. Giáo viên thấy kèm bài nộp của từng
// học sinh; học sinh chỉ thấy bài nộp của mình.
func (s *service) GetAssignments(ctx context.Context, userID string, classroomID uint, query dto.AssignmentsQuery) ([]*dto.AssignmentResponse, int64, *httpx.HTTPError) {
	classroom, err := s.repo.GetClassroomByID(ctx, classroomID)
	if err != nil {
		return nil, 0, httpx.FromError(err, errClassroomServiceFailed)
	}

	isTeacher := classroom.TeacherID == userID
	if !isTeacher {
		approved, err := s.repo.IsStudentApprovedInClassroom(ctx, classroomID, userID)
		if err != nil {
			return nil, 0, httpx.FromError(err, errClassroomServiceFailed)
		}
		if !approved {
			return nil, 0, ErrClassroomForbidden
		}
	}

	// Bài tập đã tạm dừng vẫn được liệt kê để học sinh xem lại kết quả cũ; trạng thái trong
	// response chặn việc bắt đầu lượt làm mới.
	assignments, total, err := s.repo.GetAssignmentsByClassroom(ctx, classroomID, query.PaginationQuery)
	if err != nil {
		return nil, 0, httpx.FromError(err, errClassroomServiceFailed)
	}

	userIDs := make([]string, 0)
	seenUserIDs := make(map[string]struct{})
	if isTeacher {
		for _, assignment := range assignments {
			for _, submission := range assignment.Submissions {
				if _, ok := seenUserIDs[submission.StudentID]; !ok {
					seenUserIDs[submission.StudentID] = struct{}{}
					userIDs = append(userIDs, submission.StudentID)
				}
			}
		}
	} else {
		userIDs = append(userIDs, userID)
	}

	var users []*identityClient.UserInfo
	if len(userIDs) > 0 {
		usersResp, err := s.identityClient.GetUsersInfo(ctx, &identityClient.GetUsersInfoRequest{Ids: userIDs})
		if err != nil {
			return nil, 0, httpx.FromError(err, errClassroomServiceFailed)
		}
		users = usersResp.Users
	}

	currentUserID := userID
	if isTeacher {
		currentUserID = ""
	}
	responses := dto.ToAssignmentResponses(assignments, time.Now(), currentUserID, isTeacher, users)
	return responses, total, nil
}

// GetAssignment trả chi tiết một bài tập của lớp; giáo viên thấy bài nộp của cả lớp, học sinh
// chỉ thấy của mình.
func (s *service) GetAssignment(ctx context.Context, userID string, classroomID uint, assignmentID uint) (*dto.AssignmentResponse, *httpx.HTTPError) {
	classroom, err := s.repo.GetClassroomByID(ctx, classroomID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}

	isTeacher := classroom.TeacherID == userID
	if !isTeacher {
		approved, err := s.repo.IsStudentApprovedInClassroom(ctx, classroomID, userID)
		if err != nil {
			return nil, httpx.FromError(err, errClassroomServiceFailed)
		}
		if !approved {
			return nil, ErrClassroomForbidden
		}
	}

	assignment, err := s.repo.GetAssignmentByClassroom(ctx, classroomID, assignmentID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}

	userIDs := make([]string, 0)
	seenUserIDs := make(map[string]struct{})
	if isTeacher {
		for _, submission := range assignment.Submissions {
			if _, ok := seenUserIDs[submission.StudentID]; !ok {
				seenUserIDs[submission.StudentID] = struct{}{}
				userIDs = append(userIDs, submission.StudentID)
			}
		}
	} else {
		userIDs = append(userIDs, userID)
	}

	var users []*identityClient.UserInfo
	if len(userIDs) > 0 {
		usersResp, err := s.identityClient.GetUsersInfo(ctx, &identityClient.GetUsersInfoRequest{Ids: userIDs})
		if err != nil {
			return nil, httpx.FromError(err, errClassroomServiceFailed)
		}
		users = usersResp.Users
	}

	currentUserID := userID
	if isTeacher {
		currentUserID = ""
	}
	return dto.ToAssignmentResponse(assignment, time.Now(), currentUserID, isTeacher, users), nil
}

// defaultMaxViolations là số lần vi phạm được phép mặc định: tha hai lần rời màn (có thể vô tình),
// lần thứ ba mới đình chỉ.
const defaultMaxViolations = 2

// maxViolationsOrDefault: không gửi hoặc gửi giá trị < 1 thì dùng mặc định, vì cho phép 0 lần
// vi phạm thì lần rời màn đầu tiên đã bị đình chỉ.
func maxViolationsOrDefault(value *int) int {
	if value == nil || *value < 1 {
		return defaultMaxViolations
	}
	return *value
}
