package service

import (
	"context"
	"errors"
	"net/http"
	"strings"

	identityClient "ego/api/gen/go/identity"
	"ego/platform/httpx"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"
	"ego/services/learning/internal/socket"

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
		affected = append(affected, &dto.AffectedAssignmentResponse{ID: related.ID, ClassroomID: related.ClassroomID, ClassroomName: related.Classroom.Name, Active: related.Active})
	}
	return &dto.EditableAssignmentResponse{ID: assignment.ID, ExamID: assignment.ExamID, Title: exam.Title, Description: exam.Description, Questions: dto.ToEditableAssignmentQuestions(exam), HasAttempts: hasAttempts, ActiveAttempts: activeAttempts, SubmittedAttempts: submittedAttempts, AffectedAssignments: affected}, nil
}

func (s *service) UpdateAssignmentContent(ctx context.Context, teacherID string, classroomID uint, assignmentID uint, req dto.UpdateAssignmentContentRequest) (*dto.EditableAssignmentResponse, *httpx.HTTPError) {
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

	var exam *model.Exam
	var events []pendingEvent
	// Tạm dừng các assignment dùng chung đề, nộp bài đang làm, sửa/đính chính đề
	// và bật lại assignment đều nằm trong một transaction: đề sửa không hợp lệ thì
	// assignment không bị kẹt ở trạng thái tạm dừng như khi còn gọi qua gRPC.
	err = s.repo.Transaction(ctx, func(repo repository.Repository) error {
		assignments, err := repo.GetAssignmentsByExamID(ctx, assignment.ExamID)
		if err != nil {
			return err
		}
		activeStates := make(map[uint]bool, len(assignments))
		var totalAttempts int64
		for _, related := range assignments {
			if related.Classroom.TeacherID != teacherID {
				return ErrClassroomForbidden
			}
			activeStates[related.ID] = related.Active
			_, totalCount, err := repo.CountAssignmentAttempts(ctx, related.ID)
			if err != nil {
				return err
			}
			totalAttempts += totalCount
		}
		if totalAttempts > 0 && !req.Confirm {
			return ErrAssignmentContentConfirmation
		}
		if totalAttempts > 0 && strings.TrimSpace(req.Reason) == "" {
			return httpx.NewError(http.StatusBadRequest, "[ERROR] A reason is required when changing an exam with attempts", nil)
		}
		if err := repo.UpdateAssignmentsContentAndActive(ctx, assignment.ExamID, assignment.Title, assignment.Description, false); err != nil {
			return err
		}
		txService := s.withRepoDeferringEvents(repo, &events)
		if totalAttempts > 0 {
			for _, related := range assignments {
				if _, err := txService.submitAssignmentAttempts(ctx, related.ID); err != nil {
					return err
				}
			}
		}

		questions := make([]dto.UpdatePrivateExamQuestionRequest, len(req.Questions))
		for i, question := range req.Questions {
			questions[i] = dto.UpdatePrivateExamQuestionRequest{ID: question.ID, Content: strings.TrimSpace(question.Content), Explanation: question.Explanation, Options: question.Options, CorrectOptionIndex: question.CorrectOptionIndex, ExcludedFromScore: question.ExcludedFromScore, OptionIDs: question.OptionIDs}
		}
		exam, err = txService.updatePrivateExam(ctx, assignment.ExamID, dto.UpdatePrivateExamRequest{Title: strings.TrimSpace(req.Title), Description: req.Description, Questions: questions, CorrectionOnly: totalAttempts > 0, ActorID: teacherID, Reason: strings.TrimSpace(req.Reason)})
		if err != nil {
			return err
		}

		// Đề đã có bài làm thì giữ các assignment ở trạng thái tạm dừng; chưa có thì khôi phục.
		finalActive := totalAttempts == 0
		if err := repo.UpdateAssignmentsContentAndActive(ctx, assignment.ExamID, exam.Title, exam.Description, finalActive); err != nil {
			return err
		}
		if totalAttempts == 0 {
			return repo.RestoreAssignmentActiveStates(ctx, activeStates)
		}
		return nil
	})
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	s.publishPendingEvents(events)
	return &dto.EditableAssignmentResponse{ID: assignment.ID, ExamID: assignment.ExamID, Title: exam.Title, Description: exam.Description, Questions: dto.ToEditableAssignmentQuestions(exam)}, nil
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
