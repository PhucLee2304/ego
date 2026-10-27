package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"ego/platform/httpx"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"
	"ego/services/learning/internal/socket"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Thao tác giám sát của giáo viên (PLAN_ATTEMPT_INTEGRITY.md mục 1.1, 1.2, 1.4, 1.5).
// Mọi thao tác ghi khóa attempt FOR UPDATE trước (mục 1.6).

// teacherAssignment kiểm caller là giáo viên của lớp và assignment thuộc lớp.
func (s *service) teacherAssignment(ctx context.Context, teacherID string, classroomID, assignmentID uint) (*model.Assignment, error) {
	classroom, err := s.repo.GetClassroomByID(ctx, classroomID)
	if err != nil {
		return nil, err
	}
	if classroom.TeacherID != teacherID {
		return nil, ErrClassroomForbidden
	}
	return s.repo.GetAssignmentByClassroom(ctx, classroomID, assignmentID)
}

func (s *service) GetIntegrityEvents(ctx context.Context, teacherID string, classroomID, assignmentID uint, query dto.IntegrityEventsQuery) ([]*dto.AttemptIntegrityEventResponse, int, *httpx.HTTPError) {
	level := strings.ToUpper(strings.TrimSpace(query.Level))
	switch model.IntegrityLevel(level) {
	case "", model.IntegrityLevelLog, model.IntegrityLevelWarning, model.IntegrityLevelViolation:
	default:
		return nil, 0, ErrIntegrityLevelInvalid
	}
	assignment, err := s.teacherAssignment(ctx, teacherID, classroomID, assignmentID)
	if err != nil {
		return nil, 0, httpx.FromError(err, errClassroomServiceFailed)
	}
	filter := repository.IntegrityEventFilter{StudentID: strings.TrimSpace(query.StudentID), Pending: query.Pending, Level: level}
	rows, total, err := s.repo.GetAssignmentIntegrityEvents(ctx, assignment.ID, filter, query.Limit(), query.Offset())
	if err != nil {
		return nil, 0, httpx.FromError(err, errClassroomServiceFailed)
	}
	responses := make([]*dto.AttemptIntegrityEventResponse, len(rows))
	for i, row := range rows {
		responses[i] = dto.ToAttemptIntegrityEventResponse(&row.AttemptIntegrityEvent, row.StudentID)
	}
	return responses, httpx.ToPageCounts(total, query.PageSize), nil
}

// ReviewIntegrityEvent: giáo viên bấm "Cho tiếp tục làm bài" (CONTINUE) hoặc Cấm (BAN)
// trên thẻ vi phạm. Review idempotent: cùng action → 200, đổi action → 409. CONTINUE
// không giảm ViolationCount (Q5).
func (s *service) ReviewIntegrityEvent(ctx context.Context, teacherID string, classroomID, assignmentID uint, eventID string, req dto.ReviewIntegrityEventRequest) (*dto.AttemptIntegrityResponse, *httpx.HTTPError) {
	assignment, err := s.teacherAssignment(ctx, teacherID, classroomID, assignmentID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	review, serverType := model.IntegrityReviewContinueAllowed, model.IntegrityEventContinueAllowed
	if req.Action == "BAN" {
		review, serverType = model.IntegrityReviewBanned, model.IntegrityEventTeacherBanned
	}

	var response *dto.AttemptIntegrityResponse
	var events []pendingEvent
	err = s.repo.Transaction(ctx, func(repo repository.Repository) error {
		row, err := repo.GetAssignmentIntegrityEvent(ctx, assignment.ID, eventID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrIntegrityEventNotFound
			}
			return err
		}
		attempt, err := repo.GetAttemptForIntegrity(ctx, row.AttemptID)
		if err != nil {
			return err
		}
		event, err := repo.GetIntegrityEvent(ctx, attempt.ID, eventID)
		if err != nil {
			return err
		}
		if event.Level != model.IntegrityLevelViolation {
			return ErrIntegrityEventNotViolation
		}
		if event.Review != nil {
			if *event.Review != review {
				return ErrIntegrityReviewConflict
			}
			response = dto.ToAttemptIntegrityResponse(attempt)
			return nil
		}
		if review == model.IntegrityReviewBanned && attempt.Status != model.AttemptStatusInProgress {
			return ErrAttemptAlreadySubmitted
		}

		now := s.clock()
		ok, err := repo.MarkIntegrityEventReviewed(ctx, event.ID, review, teacherID, now)
		if err != nil {
			return err
		}
		if !ok {
			return ErrIntegrityReviewConflict
		}
		event.Review, event.ReviewedAt, event.ReviewedBy = &review, &now, &teacherID
		if review == model.IntegrityReviewBanned {
			if attempt, err = s.terminateAttempt(ctx, repo, attempt, model.IntegrityTerminationTeacherBanned, teacherID, &events); err != nil {
				return err
			}
		}
		if err := s.queueIntegrityUpdate(ctx, repo, attempt, event, &events); err != nil {
			return err
		}
		detail := event.EventID
		if err := s.recordServerEvent(ctx, repo, attempt, serverType, model.IntegrityLevelLog, nil, &detail, &events); err != nil {
			return err
		}
		response = dto.ToAttemptIntegrityResponse(attempt)
		return nil
	})
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	s.publishPendingEvents(events)
	return response, nil
}

// lockStudentAttempt khóa attempt giám sát của học sinh trong assignment.
func lockStudentAttempt(ctx context.Context, repo repository.Repository, assignmentID uint, studentID string) (*model.Attempt, error) {
	attempt, err := repo.GetAssignmentAttemptForIntegrity(ctx, assignmentID, studentID)
	if err != nil {
		return nil, err
	}
	if !attempt.Proctored {
		return nil, ErrAttemptNotProctored
	}
	return attempt, nil
}

// BanAttempt: Cấm không gắn với thẻ vi phạm nào (vd từ dòng WARNING).
func (s *service) BanAttempt(ctx context.Context, teacherID string, classroomID, assignmentID uint, studentID string, req dto.BanAttemptRequest) (*dto.AttemptIntegrityResponse, *httpx.HTTPError) {
	assignment, err := s.teacherAssignment(ctx, teacherID, classroomID, assignmentID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	var response *dto.AttemptIntegrityResponse
	var events []pendingEvent
	err = s.repo.Transaction(ctx, func(repo repository.Repository) error {
		attempt, err := lockStudentAttempt(ctx, repo, assignment.ID, studentID)
		if err != nil {
			return err
		}
		if attempt.Status != model.AttemptStatusInProgress {
			return ErrAttemptAlreadySubmitted
		}
		if attempt, err = s.terminateAttempt(ctx, repo, attempt, model.IntegrityTerminationTeacherBanned, teacherID, &events); err != nil {
			return err
		}
		if err := s.recordServerEvent(ctx, repo, attempt, model.IntegrityEventTeacherBanned, model.IntegrityLevelLog, nil, trimmedNote(req.Note), &events); err != nil {
			return err
		}
		response = dto.ToAttemptIntegrityResponse(attempt)
		return nil
	})
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	s.publishPendingEvents(events)
	return response, nil
}

// ReleaseAttemptSession: "Giải phóng phiên" khi học sinh báo kẹt; xóa chủ phiên để
// claim tiếp theo được nhận ngay (epoch + 1), phiên cũ bị thu hồi.
func (s *service) ReleaseAttemptSession(ctx context.Context, teacherID string, classroomID, assignmentID uint, studentID string) (*dto.AttemptIntegrityResponse, *httpx.HTTPError) {
	assignment, err := s.teacherAssignment(ctx, teacherID, classroomID, assignmentID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	var response *dto.AttemptIntegrityResponse
	var events []pendingEvent
	var attemptID uint
	err = s.repo.Transaction(ctx, func(repo repository.Repository) error {
		attempt, err := lockStudentAttempt(ctx, repo, assignment.ID, studentID)
		if err != nil {
			return err
		}
		if attempt.Status != model.AttemptStatusInProgress {
			return ErrAttemptNotActive
		}
		attemptID = attempt.ID
		attempt.SessionTokenHash = nil
		attempt.SessionLastSeenAt = nil
		if err := repo.UpdateAttemptSession(ctx, attempt); err != nil {
			return err
		}
		if err := s.recordServerEvent(ctx, repo, attempt, model.IntegrityEventSessionReleased, model.IntegrityLevelLog, nil, nil, &events); err != nil {
			return err
		}
		events = append(events, pendingEvent{userID: attempt.UserID, payload: socket.AttemptEvent{
			Type:    socket.EventAttemptSessionRevoked,
			Payload: socket.AttemptSessionRevokedPayload{AttemptID: attempt.ID, SessionEpoch: attempt.SessionEpoch + 1},
		}})
		response = dto.ToAttemptIntegrityResponse(attempt)
		return nil
	})
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	s.integrity.clearAttempt(attemptID)
	s.publishPendingEvents(events)
	return response, nil
}

// ReinstateAttempt: "Khôi phục kết quả" khi đình chỉ oan (Q10). Bài vẫn đã nộp,
// Score = RawScore; history, submission và gradebook cập nhật cùng tx.
func (s *service) ReinstateAttempt(ctx context.Context, teacherID string, classroomID, assignmentID uint, studentID string, req dto.ReinstateAttemptRequest) (*dto.AttemptIntegrityResponse, *httpx.HTTPError) {
	assignment, err := s.teacherAssignment(ctx, teacherID, classroomID, assignmentID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	note := strings.TrimSpace(req.Note)
	var response *dto.AttemptIntegrityResponse
	var events []pendingEvent
	var gradebook *gradebookEvent
	err = s.repo.Transaction(ctx, func(repo repository.Repository) error {
		locked, err := lockStudentAttempt(ctx, repo, assignment.ID, studentID)
		if err != nil {
			return err
		}
		if locked.IntegrityOutcome != model.IntegrityOutcomeTerminated {
			return ErrAttemptNotTerminated
		}
		attempt, err := repo.GetAttemptForLifecycle(ctx, locked.ID, locked.UserID)
		if err != nil {
			return err
		}
		now := s.clock()
		attempt.IntegrityOutcome = model.IntegrityOutcomeReinstated
		attempt.ReinstatedAt, attempt.ReinstatedBy, attempt.ReinstateNote = &now, &teacherID, &note
		correct, raw := 0, 0.0
		if attempt.CorrectAnswers != nil {
			correct = *attempt.CorrectAnswers
		}
		if attempt.RawScore != nil {
			raw = *attempt.RawScore
		}
		applyScore(attempt, correct, raw)
		if err := repo.UpdateAttemptIntegrity(ctx, attempt); err != nil {
			return err
		}
		data, err := dto.MarshalAttemptHistory(attempt)
		if err != nil {
			return err
		}
		if err := repo.UpsertHistory(ctx, &model.History{Data: datatypes.JSON(data), AttemptID: attempt.ID, ExamTitle: attempt.Exam.Title, ExamType: attempt.Exam.Type, ExamYear: attempt.Exam.Year}); err != nil {
			return err
		}
		if gradebook, err = updateAssignmentSubmissionResult(ctx, repo, attempt); err != nil {
			return err
		}
		if err := createNotificationOutbox(ctx, repo, model.NotificationSourceAttempt, fmt.Sprintf("attempt:%d:reinstated:%d", attempt.ID, now.UnixNano()), []string{attempt.UserID},
			"Assignment result restored", "Your teacher restored the result of your assignment", "ATTEMPT", attempt.ID, 0, time.Time{}); err != nil {
			return err
		}
		if err := s.recordServerEvent(ctx, repo, attempt, model.IntegrityEventResultReinstated, model.IntegrityLevelLog, nil, &note, &events); err != nil {
			return err
		}
		response = dto.ToAttemptIntegrityResponse(attempt)
		return nil
	})
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	if gradebook != nil {
		s.publishGradebookEvents([]*gradebookEvent{gradebook})
	}
	s.publishPendingEvents(events)
	return response, nil
}

func trimmedNote(note *string) *string {
	if note == nil {
		return nil
	}
	value := strings.TrimSpace(*note)
	if value == "" {
		return nil
	}
	return &value
}
