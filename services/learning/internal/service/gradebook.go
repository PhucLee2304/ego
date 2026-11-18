package service

import (
	"context"
	"errors"
	"time"

	identityClient "ego/api/gen/go/identity"
	"ego/platform/httpx"
	"ego/platform/logger"
	"ego/platform/valuex"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"

	"gorm.io/gorm"
)

// GetGradebook trả bảng điểm của một bài tập cho giáo viên: mọi học sinh hiện có trong lớp, kể
// cả người chưa làm, cùng học sinh đã rời lớp nhưng có bài nộp, để không mất kết quả cũ.
func (s *service) GetGradebook(ctx context.Context, teacherID string, classroomID uint, assignmentID uint) (*dto.AssignmentResponse, *httpx.HTTPError) {
	classroom, err := s.repo.GetClassroomByID(ctx, classroomID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	if classroom.TeacherID != teacherID {
		return nil, ErrClassroomForbidden
	}

	assignment, err := s.repo.GetAssignmentByClassroom(ctx, classroomID, assignmentID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}

	members, err := s.repo.GetApprovedMembersByClassroom(ctx, classroomID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	userIDs := make([]string, 0, len(members))
	seenUserIDs := make(map[string]struct{}, len(members))
	for _, member := range members {
		if _, ok := seenUserIDs[member.UserID]; !ok {
			seenUserIDs[member.UserID] = struct{}{}
			userIDs = append(userIDs, member.UserID)
		}
	}
	for _, submission := range assignment.Submissions {
		if _, ok := seenUserIDs[submission.StudentID]; !ok {
			seenUserIDs[submission.StudentID] = struct{}{}
			userIDs = append(userIDs, submission.StudentID)
		}
	}

	var users []*identityClient.UserInfo
	if len(userIDs) > 0 {
		usersResp, err := s.identityClient.GetUsersInfo(ctx, &identityClient.GetUsersInfoRequest{Ids: userIDs})
		if err != nil {
			return nil, httpx.FromError(err, errClassroomServiceFailed)
		}
		users = usersResp.Users
	}

	now := time.Now()
	response := dto.ToAssignmentResponse(assignment, now, "", true, users)
	if len(response.Submissions) > 0 {
		summaries, err := s.repo.GetAssignmentIntegritySummaries(ctx, assignment.ID)
		if err != nil {
			return nil, httpx.FromError(err, errClassroomServiceFailed)
		}
		byAttempt := make(map[uint]*model.AttemptIntegritySummary, len(summaries))
		for _, summary := range summaries {
			byAttempt[summary.AttemptID] = summary
		}
		for _, submission := range response.Submissions {
			dto.ApplySubmissionIntegrity(submission, byAttempt[submission.AttemptID])
		}
	}
	usersByID := dto.ToUserInfoMap(users)
	submittedStudents := make(map[string]struct{}, len(response.Submissions))
	for _, submission := range response.Submissions {
		if submission.Student != nil {
			submittedStudents[submission.Student.ID] = struct{}{}
		}
	}
	for _, member := range members {
		if _, ok := submittedStudents[member.UserID]; ok {
			continue
		}
		response.Submissions = append(response.Submissions, &dto.AssignmentSubmissionResponse{
			AssignmentID: response.ID,
			Student:      usersByID[member.UserID],
			Status:       model.Status(nil, now, response.DueAt),
		})
	}
	return response, nil
}

// GetClassGradebook trả bảng điểm tổng hợp của cả lớp theo khoảng thời gian: mỗi học sinh một
// dòng với tiến độ qua các bài tập, cùng các chỉ số chung của lớp.
func (s *service) GetClassGradebook(ctx context.Context, teacherID string, classroomID uint, query dto.GradebookRangeQuery) (*dto.ClassGradebookResponse, *httpx.HTTPError) {
	classroom, err := s.repo.GetClassroomByID(ctx, classroomID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	if classroom.TeacherID != teacherID {
		return nil, ErrClassroomForbidden
	}
	assignments, err := s.repo.GetAssignmentsForGradebook(ctx, classroomID, query.From, query.To)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	members, err := s.repo.GetApprovedMembersByClassroom(ctx, classroomID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	ids := make([]string, len(members))
	for i, member := range members {
		ids[i] = member.UserID
	}
	usersByID := s.usersInfo(ctx, ids...)
	now := time.Now()
	response := &dto.ClassGradebookResponse{ClassroomID: classroomID, Assignments: make([]*dto.GradebookAssignment, len(assignments)), Students: make([]*dto.StudentProgressResponse, 0, len(members))}
	for i, assignment := range assignments {
		response.Assignments[i] = &dto.GradebookAssignment{ID: assignment.ID, Title: assignment.Title, DueAt: assignment.DueAt}
	}
	var totalScore float64
	var scored, completed, total int
	for _, member := range members {
		progress, sum := studentProgress(usersByID[member.UserID], assignments, now)
		response.Students = append(response.Students, progress)
		total += len(assignments)
		completed += sum.completed
		scored += sum.scored
		totalScore += sum.score
	}
	if scored > 0 {
		value := valuex.RoundFloat(totalScore/float64(scored), 2)
		response.AverageScore = &value
	}
	if total > 0 {
		response.CompletionRate = valuex.RoundFloat(float64(completed)*100/float64(total), 2)
	}
	return response, nil
}

// GetStudentProgress trả tiến độ bài tập của một học sinh; giáo viên xem của bất kỳ học sinh nào,
// học sinh chỉ xem được của chính mình.
func (s *service) GetStudentProgress(ctx context.Context, userID string, classroomID uint, studentID string) (*dto.StudentProgressResponse, *httpx.HTTPError) {
	classroom, err := s.repo.GetClassroomByID(ctx, classroomID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	if classroom.TeacherID != userID && userID != studentID {
		return nil, ErrClassroomForbidden
	}
	approved, err := s.repo.IsStudentApprovedInClassroom(ctx, classroomID, studentID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	if !approved {
		return nil, ErrClassroomForbidden
	}
	// Chỉ tính cho một học sinh: không dựng cả bảng điểm lớp và không hỏi identity cho cả lớp.
	assignments, err := s.repo.GetAssignmentsForGradebook(ctx, classroomID, nil, nil)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	progress, _ := studentProgress(s.usersInfo(ctx, studentID)[studentID], assignments, time.Now())
	return progress, nil
}

// progressSum là tổng của một học sinh để GetClassGradebook cộng dồn cho cả lớp.
type progressSum struct {
	completed, scored int
	score             float64
}

// studentProgress tính tiến độ của một học sinh trên các assignment (đã preload submissions).
func studentProgress(student *dto.UserInfo, assignments []*model.Assignment, now time.Time) (*dto.StudentProgressResponse, progressSum) {
	progress := &dto.StudentProgressResponse{Student: student, Assignments: make([]*dto.StudentAssignmentProgress, 0, len(assignments))}
	var sum progressSum
	for _, assignment := range assignments {
		var found *model.AssignmentSubmission
		for i := range assignment.Submissions {
			if assignment.Submissions[i].StudentID == student.ID {
				found = &assignment.Submissions[i]
				break
			}
		}
		status := model.Status(found, now, assignment.DueAt)
		cell := &dto.StudentAssignmentProgress{AssignmentID: assignment.ID, Title: assignment.Title, DueAt: assignment.DueAt, Status: status}
		if found != nil {
			cell.Score = found.Score
		}
		progress.Assignments = append(progress.Assignments, cell)
		if status == model.SubmissionStatusSubmitted {
			sum.completed++
		}
		if cell.Score != nil {
			sum.score += *cell.Score
			sum.scored++
		}
	}
	if sum.scored > 0 {
		value := valuex.RoundFloat(sum.score/float64(sum.scored), 2)
		progress.AverageScore = &value
	}
	if len(assignments) > 0 {
		progress.CompletionRate = valuex.RoundFloat(float64(sum.completed)*100/float64(len(assignments)), 2)
	}
	return progress, sum
}

// GetAssignmentAnalytics thống kê một bài tập từ bảng điểm: số học sinh theo trạng thái nộp,
// điểm thấp nhất, cao nhất, trung bình.
func (s *service) GetAssignmentAnalytics(ctx context.Context, teacherID string, classroomID uint, assignmentID uint) (*dto.AssignmentAnalyticsResponse, *httpx.HTTPError) {
	gradebook, httpErr := s.GetGradebook(ctx, teacherID, classroomID, assignmentID)
	if httpErr != nil {
		return nil, httpErr
	}
	response := &dto.AssignmentAnalyticsResponse{AssignmentID: assignmentID, TotalStudents: len(gradebook.Submissions)}
	var sum float64
	var scored int
	for _, submission := range gradebook.Submissions {
		switch submission.Status {
		case model.SubmissionStatusNotStarted:
			response.NotStarted++
		case model.SubmissionStatusInProgress:
			response.InProgress++
		case model.SubmissionStatusSubmitted:
			response.Submitted++
		case model.SubmissionStatusLate:
			response.Late++
		}
		if submission.Score != nil {
			score := *submission.Score
			sum += score
			scored++
			if response.MinimumScore == nil || score < *response.MinimumScore {
				value := score
				response.MinimumScore = &value
			}
			if response.MaximumScore == nil || score > *response.MaximumScore {
				value := score
				response.MaximumScore = &value
			}
		}
	}
	if scored > 0 {
		value := valuex.RoundFloat(sum/float64(scored), 2)
		response.AverageScore = &value
	}
	if response.TotalStudents > 0 {
		response.CompletionRate = valuex.RoundFloat(float64(response.Submitted)*100/float64(response.TotalStudents), 2)
	}
	// Trước đây gọi RPC GetAssignmentQuestionAnalytics của exams; giờ đọc repo trực tiếp.
	questions, err := s.repo.GetAssignmentQuestionAnalytics(ctx, assignmentID, gradebook.ExamID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	for _, question := range questions {
		item := &dto.QuestionAnalyticsResponse{QuestionID: question.QuestionID, Content: question.Content, Order: question.Order, AnsweredCount: question.AnsweredCount, WrongCount: question.WrongCount}
		if item.AnsweredCount > 0 {
			item.WrongRate = valuex.RoundFloat(float64(item.WrongCount)*100/float64(item.AnsweredCount), 2)
		}
		response.Questions = append(response.Questions, item)
	}
	return response, nil
}

// ReconcileAssignmentSubmissions đồng bộ thẳng attempts đã nộp → submissions trong
// một tx (trước đây xếp outbox để exams gửi lại sang classrooms). Trả số submission
// đã sửa; vì điểm luôn ghi cùng tx với bài nộp nên thường là 0.
func (s *service) ReconcileAssignmentSubmissions(ctx context.Context, teacherID string, classroomID uint, assignmentID uint) (int, *httpx.HTTPError) {
	// Bản kèm Classroom, không preload submissions (đọc trong tx bên dưới).
	assignment, err := s.repo.GetAssignmentForAttemptValidation(ctx, assignmentID)
	// Classroom.ID = 0: lớp đã xóa mềm nên Preload trả rỗng; coi như không tồn tại.
	if err == nil && (assignment.ClassroomID != classroomID || assignment.Classroom.ID == 0) {
		err = gorm.ErrRecordNotFound
	}
	if err != nil {
		return 0, httpx.FromError(err, errClassroomServiceFailed)
	}
	if assignment.Classroom.TeacherID != teacherID {
		return 0, ErrClassroomForbidden
	}

	var events []*gradebookEvent
	err = s.repo.Transaction(ctx, func(repo repository.Repository) error {
		attempts, err := repo.GetSubmittedAttemptsByAssignment(ctx, assignmentID)
		if err != nil {
			return err
		}
		submissions, err := repo.GetAssignmentSubmissions(ctx, assignmentID)
		if err != nil {
			return err
		}
		byStudent := make(map[string]*model.AssignmentSubmission, len(submissions))
		for _, submission := range submissions {
			byStudent[submission.StudentID] = submission
		}
		for _, attempt := range attempts {
			submission := byStudent[attempt.UserID]
			if submission != nil && submission.AttemptID == attempt.ID && submission.Score != nil && *submission.Score == valuex.RoundFloat(*attempt.Score, 2) {
				continue
			}
			event, err := writeAssignmentSubmissionResult(ctx, repo, assignment, attempt)
			if errors.Is(err, ErrInvalidSubmissionScore) {
				// Một bài điểm hỏng không chặn đồng bộ các bài còn lại.
				logger.Log.Error().Uint("attempt_id", attempt.ID).Float64("score", *attempt.Score).Msg("[ERROR] Skipped attempt with invalid submission score")
				continue
			}
			if err != nil {
				return err
			}
			events = append(events, event)
		}
		return nil
	})
	if err != nil {
		return 0, httpx.FromError(err, errClassroomServiceFailed)
	}
	s.publishGradebookEvents(events)
	return len(events), nil
}
