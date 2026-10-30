package service

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	identityClient "ego/api/gen/go/identity"
	"ego/platform/httpx"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"
	"ego/services/learning/internal/socket"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func (s *service) GetEditableAssignment(ctx context.Context, teacherID string, classroomID uint, assignmentID uint) (*dto.EditableAssignmentResponse, *httpx.HTTPError) {
	assignment, err := s.repo.GetAssignmentByClassroom(ctx, classroomID, assignmentID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	classroom, err := s.repo.GetClassroomByID(ctx, classroomID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	if classroom.TeacherID != teacherID {
		return nil, ErrClassroomForbidden
	}
	exam, err := s.getPrivateExam(ctx, assignment.ExamID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	relatedAssignments, err := s.repo.GetAssignmentsByExamID(ctx, assignment.ExamID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	hasAttempts := false
	var activeAttempts int64
	var submittedAttempts int64
	affected := make([]*dto.AffectedAssignmentResponse, 0, len(relatedAssignments))
	for _, related := range relatedAssignments {
		activeCount, totalCount, err := s.repo.CountAssignmentAttempts(ctx, related.ID)
		if err != nil {
			return nil, httpx.FromError(err, errClassroomServiceFailed)
		}
		activeAttempts += activeCount
		submittedAttempts += totalCount - activeCount
		hasAttempts = hasAttempts || totalCount > 0
		affected = append(affected, &dto.AffectedAssignmentResponse{ID: related.ID, ClassroomID: related.ClassroomID, ClassroomName: related.Classroom.Name, Active: related.Active, Duration: related.Duration, OpensAt: related.OpensAt, DueAt: related.DueAt, Proctored: related.Proctored, MaxViolations: related.MaxViolations})
	}
	return &dto.EditableAssignmentResponse{ID: assignment.ID, ExamID: assignment.ExamID, Title: exam.Title, Description: exam.Description, Questions: dto.ToEditableAssignmentQuestions(exam), HasAttempts: hasAttempts, ActiveAttempts: activeAttempts, SubmittedAttempts: submittedAttempts, AffectedAssignments: affected}, nil
}

// UpdateAssignment lưu màn sửa bài trong một transaction. Client gửi cả form như lúc tạo;
// backend so với dữ liệu hiện có và chỉ ghi phần thay đổi: nội dung đề, lịch của lớp đã có
// bài, rồi giao thêm lớp chưa có. Một bước lỗi hay cần xác nhận (409) thì không bước nào
// được ghi; sự kiện socket chỉ publish sau khi commit.
func (s *service) UpdateAssignment(ctx context.Context, teacherID string, classroomID uint, assignmentID uint, req dto.UpdateAssignmentRequest) (*dto.AssignmentResponse, *httpx.HTTPError) {
	var response *dto.AssignmentResponse
	var events []pendingEvent
	err := s.repo.Transaction(ctx, func(repo repository.Repository) error {
		classroom, err := repo.GetClassroomByID(ctx, classroomID)
		if err != nil {
			return err
		}
		if classroom.TeacherID != teacherID {
			return ErrClassroomForbidden
		}
		assignment, err := repo.GetAssignmentByClassroom(ctx, classroomID, assignmentID)
		if err != nil {
			return err
		}
		exam, err := repo.GetPrivateAssignmentExam(ctx, assignment.ExamID)
		if err != nil {
			return err
		}
		// Mọi thay đổi (đề, lịch, lớp giao thêm) gom vào một dòng lịch sử. Đọc lịch cũ
		// trước khi sửa đề vì sửa đề có thể tạm dừng/bật lại assignment.
		related, err := repo.GetAssignmentsByExamID(ctx, assignment.ExamID)
		if err != nil {
			return err
		}
		byClassroom := make(map[uint]*model.Assignment, len(related))
		for _, item := range related {
			byClassroom[item.ClassroomID] = item
		}
		var scheduleChanges []string
		var added []dto.AssignmentScheduleRequest
		for _, schedule := range req.Schedules {
			if existing, ok := byClassroom[schedule.ClassroomID]; ok {
				scheduleChanges = append(scheduleChanges, describeScheduleChanges(existing.Classroom.Name, existing, schedule)...)
				continue
			}
			target, err := repo.GetClassroomByID(ctx, schedule.ClassroomID)
			if err != nil {
				return err
			}
			added = append(added, schedule)
			scheduleChanges = append(scheduleChanges, describeAssigned(target.Name, schedule))
		}

		if assignmentContentChanged(exam, req) {
			content := dto.UpdateAssignmentContentRequest{Title: req.Title, Description: req.Description, Questions: req.Questions, Confirm: req.Confirm, Reason: req.Reason}
			changes := append(describeContentChanges(exam, req), scheduleChanges...)
			if _, err := s.updateAssignmentContentTx(ctx, repo, &events, teacherID, assignment, content, changes); err != nil {
				return err
			}
		} else if len(scheduleChanges) > 0 {
			if err := repo.CreateExamAudit(ctx, &model.ExamAudit{ExamID: assignment.ExamID, ActorID: teacherID, Reason: strings.TrimSpace(req.Reason), Before: datatypes.JSON("{}"), After: datatypes.JSON("{}"), ScoreChanges: datatypes.JSON("[]"), Changes: auditChanges(scheduleChanges)}); err != nil {
				return err
			}
		}

		for _, schedule := range req.Schedules {
			existing, ok := byClassroom[schedule.ClassroomID]
			if !ok || !assignmentScheduleChanged(existing, schedule) {
				continue
			}
			proctored, maxViolations := schedule.Proctored, maxViolationsOrDefault(schedule.MaxViolations)
			if _, err := updateAssignmentScheduleTx(ctx, repo, teacherID, schedule.ClassroomID, existing.ID, dto.UpdateAssignmentScheduleRequest{
				Duration: schedule.Duration, OpensAt: schedule.OpensAt, DueAt: schedule.DueAt, Confirm: req.Confirm, Proctored: &proctored, MaxViolations: &maxViolations,
			}); err != nil {
				return err
			}
		}

		// Đọc lại sau khi sửa: lớp mới nhận tiêu đề mới và response phản ánh lịch mới.
		assignment, err = repo.GetAssignmentByClassroom(ctx, classroomID, assignmentID)
		if err != nil {
			return err
		}
		if len(added) > 0 {
			if _, err := assignExistingExamTx(ctx, repo, teacherID, assignment, added); err != nil {
				return err
			}
		}
		response = dto.ToAssignmentResponse(assignment, time.Now(), "", true, nil)
		return nil
	})
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	s.publishPendingEvents(events)
	return response, nil
}

// assignmentContentChanged so form sửa với đề hiện tại (bỏ khoảng trắng hai đầu như khi lưu).
// Đổi thứ tự, thêm/xoá câu hay đáp án đều tính là thay đổi.
func assignmentContentChanged(exam *model.Exam, req dto.UpdateAssignmentRequest) bool {
	text := func(value *string) string {
		if value == nil {
			return ""
		}
		return strings.TrimSpace(*value)
	}
	if strings.TrimSpace(req.Title) != strings.TrimSpace(exam.Title) || text(req.Description) != text(exam.Description) {
		return true
	}
	current := dto.ToEditableAssignmentQuestions(exam)
	if len(current) != len(req.Questions) {
		return true
	}
	for i, question := range req.Questions {
		old := current[i]
		if question.ID != old.ID || strings.TrimSpace(question.Content) != strings.TrimSpace(old.Content) ||
			text(question.Explanation) != text(old.Explanation) || question.CorrectOptionIndex != old.CorrectOptionIndex ||
			question.ExcludedFromScore != old.ExcludedFromScore || !slices.Equal(question.OptionIDs, old.OptionIDs) ||
			len(question.Options) != len(old.Options) {
			return true
		}
		for j, option := range question.Options {
			if strings.TrimSpace(option) != strings.TrimSpace(old.Options[j]) {
				return true
			}
		}
	}
	return false
}

func assignmentScheduleChanged(assignment *model.Assignment, schedule dto.AssignmentScheduleRequest) bool {
	return assignment.Duration != schedule.Duration || !assignment.OpensAt.Equal(schedule.OpensAt) ||
		!assignment.DueAt.Equal(schedule.DueAt) || assignment.Proctored != schedule.Proctored ||
		assignment.MaxViolations != maxViolationsOrDefault(schedule.MaxViolations)
}

// updateAssignmentContentTx sửa đề trong transaction của người gọi. Tạm dừng các
// assignment dùng chung đề, nộp bài đang làm, sửa/đính chính đề và bật lại
// assignment đều nằm trong transaction: đề sửa không hợp lệ thì assignment không
// bị kẹt ở trạng thái tạm dừng. Sự kiện socket được hoãn vào events, publish sau commit.
func (s *service) updateAssignmentContentTx(ctx context.Context, repo repository.Repository, events *[]pendingEvent, teacherID string, assignment *model.Assignment, req dto.UpdateAssignmentContentRequest, changes []string) (*model.Exam, error) {
	assignments, err := repo.GetAssignmentsByExamID(ctx, assignment.ExamID)
	if err != nil {
		return nil, err
	}
	activeStates := make(map[uint]bool, len(assignments))
	var totalAttempts int64
	for _, related := range assignments {
		if related.Classroom.TeacherID != teacherID {
			return nil, ErrClassroomForbidden
		}
		activeStates[related.ID] = related.Active
		_, totalCount, err := repo.CountAssignmentAttempts(ctx, related.ID)
		if err != nil {
			return nil, err
		}
		totalAttempts += totalCount
	}
	if totalAttempts > 0 && !req.Confirm {
		return nil, ErrAssignmentContentConfirmation
	}
	if totalAttempts > 0 && strings.TrimSpace(req.Reason) == "" {
		return nil, httpx.NewError(http.StatusBadRequest, "[ERROR] A reason is required when changing an exam with attempts", nil)
	}
	if err := repo.UpdateAssignmentsContentAndActive(ctx, assignment.ExamID, assignment.Title, assignment.Description, false); err != nil {
		return nil, err
	}
	txService := s.withRepoDeferringEvents(repo, events)
	if totalAttempts > 0 {
		for _, related := range assignments {
			if _, err := txService.submitAssignmentAttempts(ctx, related.ID); err != nil {
				return nil, err
			}
		}
	}

	questions := make([]dto.UpdatePrivateExamQuestionRequest, len(req.Questions))
	for i, question := range req.Questions {
		questions[i] = dto.UpdatePrivateExamQuestionRequest{ID: question.ID, Content: strings.TrimSpace(question.Content), Explanation: question.Explanation, Options: question.Options, CorrectOptionIndex: question.CorrectOptionIndex, ExcludedFromScore: question.ExcludedFromScore, OptionIDs: question.OptionIDs}
	}
	exam, err := txService.updatePrivateExam(ctx, assignment.ExamID, dto.UpdatePrivateExamRequest{Title: strings.TrimSpace(req.Title), Description: req.Description, Questions: questions, CorrectionOnly: totalAttempts > 0, ActorID: teacherID, Reason: strings.TrimSpace(req.Reason), Changes: changes})
	if err != nil {
		return nil, err
	}

	// Đề đã có bài làm thì giữ các assignment ở trạng thái tạm dừng; chưa có thì khôi phục.
	finalActive := totalAttempts == 0
	if err := repo.UpdateAssignmentsContentAndActive(ctx, assignment.ExamID, exam.Title, exam.Description, finalActive); err != nil {
		return nil, err
	}
	if totalAttempts == 0 {
		return exam, repo.RestoreAssignmentActiveStates(ctx, activeStates)
	}
	return exam, nil
}

func (s *service) ResetAssignmentAttempt(ctx context.Context, teacherID string, classroomID uint, assignmentID uint, studentID string) *httpx.HTTPError {
	assignment, err := s.repo.GetAssignmentByClassroom(ctx, classroomID, assignmentID)
	if err != nil {
		return httpx.FromError(err, errClassroomServiceFailed)
	}
	classroom, err := s.repo.GetClassroomByID(ctx, classroomID)
	if err != nil {
		return httpx.FromError(err, errClassroomServiceFailed)
	}
	if classroom.TeacherID != teacherID {
		return ErrClassroomForbidden
	}
	// Xóa attempt (kèm đáp án, history) và submission trong một transaction. Khóa
	// attempt trước như mọi luồng sửa attempt (PLAN_ATTEMPT_INTEGRITY.md mục 1.6).
	var resetAttemptID uint
	if err := s.repo.Transaction(ctx, func(repo repository.Repository) error {
		if attempt, err := repo.GetAssignmentAttemptForIntegrity(ctx, assignment.ID, studentID); err == nil {
			resetAttemptID = attempt.ID
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if _, err := repo.GetAssignmentSubmissionByAssignmentAndStudent(ctx, assignment.ID, studentID); err != nil {
			return err
		}
		if err := repo.ResetAssignmentAttempt(ctx, assignment.ID, studentID); err != nil {
			return err
		}
		return repo.DeleteAssignmentSubmission(ctx, assignment.ID, studentID)
	}); err != nil {
		return httpx.FromError(err, errClassroomServiceFailed)
	}
	// Học sinh đang làm được mở khóa ngay thay vì kẹt ở màn làm bài.
	if resetAttemptID != 0 {
		s.integrity.clearAttempt(resetAttemptID)
		if s.attemptPublisher != nil {
			s.attemptPublisher(studentID, socket.AttemptEvent{Type: socket.EventAttemptReset, Payload: socket.AttemptResetPayload{AttemptID: resetAttemptID}})
		}
	}
	return nil
}

func (s *service) GetAssignmentAudits(ctx context.Context, teacherID string, classroomID uint, assignmentID uint) ([]*dto.ExamAuditResponse, *httpx.HTTPError) {
	assignment, err := s.repo.GetAssignmentByClassroom(ctx, classroomID, assignmentID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	classroom, err := s.repo.GetClassroomByID(ctx, classroomID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	if classroom.TeacherID != teacherID {
		return nil, ErrClassroomForbidden
	}
	audits, err := s.getExamAudits(ctx, assignment.ExamID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	responses := make([]*dto.ExamAuditResponse, len(audits))
	actorIDs := make([]string, 0, len(audits))
	seenActorIDs := make(map[string]struct{}, len(audits))
	for i, audit := range audits {
		item, err := dto.ToExamAuditResponse(audit)
		if err != nil {
			return nil, httpx.FromError(err, errClassroomServiceFailed)
		}
		responses[i] = item
		if _, exists := seenActorIDs[audit.ActorID]; !exists {
			seenActorIDs[audit.ActorID] = struct{}{}
			actorIDs = append(actorIDs, audit.ActorID)
		}
	}
	if len(actorIDs) > 0 {
		usersResp, err := s.identityClient.GetUsersInfo(ctx, &identityClient.GetUsersInfoRequest{Ids: actorIDs})
		if err != nil {
			return nil, httpx.FromError(err, errClassroomServiceFailed)
		}
		usersByID := dto.ToUserInfoMap(usersResp.Users)
		for _, response := range responses {
			response.Actor = usersByID[response.ActorID]
		}
	}
	return responses, nil
}
