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

	// Chuẩn hóa chuỗi nhập vào giống request gRPC CreatePrivateExam cũ.
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
	// kỳ bước nào thì không còn đề mồ côi (thay DeletePrivateExamRollback cũ).
	if err := s.repo.Transaction(ctx, func(repo repository.Repository) error {
		exam, err := s.withRepo(repo).createPrivateExam(ctx, examReq)
		if err != nil {
			return err
		}
		assignments = make([]*model.Assignment, 0, len(req.Schedules))
		for _, schedule := range req.Schedules {
			assignments = append(assignments, &model.Assignment{
				ClassroomID: schedule.ClassroomID,
				Title:       strings.TrimSpace(req.Title),
				Description: req.Description,
				ExamID:      exam.ID,
				Duration:    schedule.Duration,
				OpensAt:     schedule.OpensAt,
				DueAt:       schedule.DueAt,
				Active:      true,
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

func (s *service) AssignExistingExam(ctx context.Context, teacherID string, classroomID uint, assignmentID uint, req dto.AssignExistingExamRequest) (*dto.CreatedAssignmentsResponse, *httpx.HTTPError) {
	source, err := s.repo.GetAssignmentByClassroom(ctx, classroomID, assignmentID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	sourceClassroom, err := s.repo.GetClassroomByID(ctx, classroomID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	if sourceClassroom.TeacherID != teacherID {
		return nil, ErrClassroomForbidden
	}

	seen := make(map[uint]struct{}, len(req.Schedules))
	for _, schedule := range req.Schedules {
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
		exists, err := s.repo.AssignmentExistsByClassroomAndExam(ctx, schedule.ClassroomID, source.ExamID)
		if err != nil {
			return nil, httpx.FromError(err, errClassroomServiceFailed)
		}
		if exists {
			return nil, ErrAssignmentAlreadyExists
		}
	}

	var assignments []*model.Assignment
	err = s.repo.Transaction(ctx, func(repo repository.Repository) error {
		assignments = make([]*model.Assignment, 0, len(req.Schedules))
		for _, schedule := range req.Schedules {
			assignments = append(assignments, &model.Assignment{
				ClassroomID: schedule.ClassroomID,
				Title:       source.Title,
				Description: source.Description,
				ExamID:      source.ExamID,
				Duration:    schedule.Duration,
				OpensAt:     schedule.OpensAt,
				DueAt:       schedule.DueAt,
				Active:      true,
			})
		}
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
	})
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	return dto.ToCreatedAssignmentsResponse(assignments), nil
}

func (s *service) GetAssignments(ctx context.Context, userID string, classroomID uint, query dto.AssignmentsQuery) ([]*dto.AssignmentResponse, int, *httpx.HTTPError) {
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

	// Inactive assignments intentionally remain visible so students can review
	// previous results; the response state prevents starting a new attempt.
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
	return responses, httpx.ToPageCounts(total, query.PageSize), nil
}

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
