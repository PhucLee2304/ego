package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"ego/platform/logger"
	platformsocket "ego/platform/socket"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"
	"ego/services/learning/internal/socket"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Giám sát làm bài. Các bất biến mọi luồng phải giữ:
//  1. Attempt chỉ chuyển từ ACTIVE sang đã nộp đúng một lần (finalizeAttempt).
//  2. Chỉ một transaction chốt kết quả cuối: mọi luồng khóa dòng attempt trước.
//  3. Một sự kiện (eventId) hay một episode (startEventId) chỉ được tính một lần.
//  4. Bài đình chỉ chưa khôi phục luôn có Score = 0 (applyScore + CHECK ở DB).
//  5. Push WebSocket và outbox chỉ phát sau khi commit, từ transaction đã chốt.

// integrityNotFound là lỗi socket chung khi attempt không tồn tại hoặc không thuộc người gọi,
// để không lộ attempt của người khác.
func integrityNotFound() error {
	return platformsocket.NewHandlerError(socket.ErrorCodeAttemptNotFound, "[SOCKET] Attempt not found")
}

// lockOwnAttempt khóa attempt và kiểm tra thuộc userID; lỗi là lỗi WS.
func lockOwnAttempt(ctx context.Context, repo repository.Repository, attemptID uint, userID string) (*model.Attempt, error) {
	attempt, err := repo.GetAttemptForIntegrity(ctx, attemptID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, integrityNotFound()
		}
		return nil, err
	}
	if attempt.UserID != userID {
		return nil, integrityNotFound()
	}
	return attempt, nil
}

// ClaimAttemptSession phục vụ WS ATTEMPT_SESSION_CLAIM: kết nối xin quyền làm bài có giám
// sát, để mỗi lúc chỉ một thiết bị/tab làm bài (xem claimResult).
func (s *service) ClaimAttemptSession(ctx context.Context, userID, connectionID string, payload socket.AttemptSessionClaimPayload) (*dto.AttemptSessionClaimResponse, error) {
	cfg := s.integrityCfg()
	var response *dto.AttemptSessionClaimResponse
	var rejected *claimDecision
	var events []pendingEvent
	var change ownerChange
	claimed := false
	err := s.repo.Transaction(ctx, func(repo repository.Repository) error {
		attempt, err := lockOwnAttempt(ctx, repo, payload.AttemptID, userID)
		if err != nil {
			return err
		}
		if !attempt.Proctored {
			return platformsocket.NewHandlerError(socket.ErrorCodeAttemptNotProctored, "[SOCKET] Attempt is not proctored")
		}
		if attempt.Status != model.AttemptStatusInProgress {
			return platformsocket.NewHandlerError(socket.ErrorCodeAttemptNotActive, ErrAttemptNotActive.Message)
		}

		now := s.clock()
		decision := decideSessionClaim(attempt, s.integrity.owner(attempt.ID), connectionID, payload.SessionToken, payload.Takeover, now, cfg)
		if !decision.Accepted {
			rejected = &decision
			if decision.Conflict {
				return s.recordServerEvent(ctx, repo, attempt, model.IntegrityEventSessionConflict, model.IntegrityLevelWarning, &connectionID, nil, &events)
			}
			return nil
		}

		token, hash, err := newSessionToken()
		if err != nil {
			return err
		}
		previousEpoch := attempt.SessionEpoch
		attempt.SessionTokenHash = &hash
		attempt.SessionEpoch = decision.Epoch
		attempt.SessionLastSeenAt = &now
		if err := repo.UpdateAttemptSession(ctx, attempt); err != nil {
			return err
		}
		if decision.Result == claimTakeover {
			if err := s.recordServerEvent(ctx, repo, attempt, model.IntegrityEventSessionTakeover, model.IntegrityLevelWarning, &connectionID, nil, &events); err != nil {
				return err
			}
		}
		if previousEpoch > 0 && decision.Epoch != previousEpoch {
			events = append(events, pendingEvent{userID: attempt.UserID, payload: socket.AttemptEvent{
				Type:    socket.EventAttemptSessionRevoked,
				Payload: socket.AttemptSessionRevokedPayload{AttemptID: attempt.ID, SessionEpoch: decision.Epoch},
			}})
		}
		// Gắn chủ phiên khi còn giữ khóa dòng attempt để các claim đồng thời áp theo đúng
		// thứ tự commit; tx lỗi thì restoreOwner trả lại chủ cũ.
		change = s.integrity.setOwner(attempt.ID, connectionID)
		claimed = true
		response = &dto.AttemptSessionClaimResponse{
			SessionToken: token,
			SessionEpoch: attempt.SessionEpoch,
			State:        dto.ToIntegrityState(attempt),
		}
		return nil
	})
	if err != nil {
		if claimed {
			s.integrity.restoreOwner(payload.AttemptID, connectionID, change)
		}
		return nil, err
	}
	s.publishPendingEvents(events)
	if rejected != nil {
		return nil, claimError(*rejected)
	}
	return response, nil
}

// claimError đổi quyết định từ chối claim sang lỗi socket; SESSION_RECOVERING kèm thời gian
// cần chờ để client tự thử lại đúng lúc.
func claimError(decision claimDecision) error {
	switch decision.Result {
	case claimRejectRecovering:
		return platformsocket.NewHandlerError(socket.ErrorCodeSessionRecovering, "[SOCKET] Previous session is reconnecting").
			WithDetails(map[string]int64{"retryAfterMs": decision.RetryAfter.Milliseconds()})
	case claimRejectTakeoverRequired:
		return platformsocket.NewHandlerError(socket.ErrorCodeSessionTakeoverRequired, "[SOCKET] Confirm to continue the attempt on this device")
	default:
		return platformsocket.NewHandlerError(socket.ErrorCodeSessionInUse, "[SOCKET] Attempt is open in another session")
	}
}

// clientIntegrityEventTypes là các loại event client được gửi; còn lại do server sinh.
var clientIntegrityEventTypes = map[model.IntegrityEventType]bool{
	model.IntegrityEventAwayStart:         true,
	model.IntegrityEventAwayEnd:           true,
	model.IntegrityEventReload:            true,
	model.IntegrityEventNavigationBlocked: true,
	model.IntegrityEventBackBlocked:       true,
	model.IntegrityEventCopy:              true,
	model.IntegrityEventPaste:             true,
}

// RecordIntegrityEvent phục vụ WS ATTEMPT_INTEGRITY_EVENT: ghi sự kiện giám sát client gửi,
// phân loại mức độ, tăng số vi phạm và đình chỉ khi vượt ngưỡng.
func (s *service) RecordIntegrityEvent(ctx context.Context, userID, connectionID string, payload socket.AttemptIntegrityEventPayload) (*dto.IntegrityEventAckResponse, error) {
	eventType := model.IntegrityEventType(strings.ToUpper(strings.TrimSpace(payload.Type)))
	eventID := strings.TrimSpace(payload.EventID)
	if !clientIntegrityEventTypes[eventType] {
		return nil, platformsocket.NewHandlerError(socket.ErrorCodeIntegrityEventInvalid, "[SOCKET] Unknown integrity event type")
	}
	if eventType == model.IntegrityEventAwayEnd && (payload.StartEventID == nil || strings.TrimSpace(*payload.StartEventID) == "") {
		return nil, platformsocket.NewHandlerError(socket.ErrorCodeIntegrityEventInvalid, "[SOCKET] AWAY_END requires startEventId")
	}

	var response *dto.IntegrityEventAckResponse
	var events []pendingEvent
	err := s.repo.Transaction(ctx, func(repo repository.Repository) error {
		attempt, err := lockOwnAttempt(ctx, repo, payload.AttemptID, userID)
		if err != nil {
			return err
		}
		if !attempt.Proctored || attempt.ContextID == nil {
			response = &dto.IntegrityEventAckResponse{EventID: eventID, Ignored: true, State: dto.ToIntegrityState(attempt)}
			return nil
		}
		if existing, err := repo.GetIntegrityEvent(ctx, attempt.ID, eventID); err == nil {
			response = integrityAck(existing, attempt, true)
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if attempt.Status != model.AttemptStatusInProgress {
			return platformsocket.NewHandlerError(socket.ErrorCodeAttemptNotActive, ErrAttemptNotActive.Message)
		}

		now := s.clock()
		event := &model.AttemptIntegrityEvent{
			AttemptID:        attempt.ID,
			EventID:          eventID,
			SessionEpoch:     payload.SessionEpoch,
			ConnectionID:     &connectionID,
			Type:             eventType,
			Level:            model.IntegrityLevelLog,
			StartEventID:     payload.StartEventID,
			ClientDurationMs: payload.ClientDurationMs,
			Replayed:         payload.Replayed,
			Stale:            s.integrity.owner(attempt.ID) != connectionID || payload.SessionEpoch != attempt.SessionEpoch,
			Detail:           payload.Detail,
			ClientAt:         payload.At,
			ReceivedAt:       now,
		}
		switch eventType {
		case model.IntegrityEventAwayStart:
			event.Level = classifyAwayStart(event, attempt)
		case model.IntegrityEventAwayEnd:
			if err := recordAwayDuration(ctx, repo, attempt, event); err != nil {
				return err
			}
		}
		if attempt, err = s.storeIntegrityEvent(ctx, repo, attempt, event, &events); err != nil {
			return err
		}
		response = integrityAck(event, attempt, false)
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.publishPendingEvents(events)
	return response, nil
}

// storeIntegrityEvent ghi event trong tx đã khóa attempt. Event mức VIOLATION thì
// tăng ViolationCount, push ATTEMPT_VIOLATION và đình chỉ khi vượt ngưỡng. Gom push
// cho giáo viên (trừ event stale). Trả attempt sau cùng (đã đọc lại nếu bị đình chỉ).
func (s *service) storeIntegrityEvent(ctx context.Context, repo repository.Repository, attempt *model.Attempt, event *model.AttemptIntegrityEvent, events *[]pendingEvent) (*model.Attempt, error) {
	if event.Level == model.IntegrityLevelViolation {
		attempt.ViolationCount++
		violationNo := attempt.ViolationCount
		event.ViolationNo = &violationNo
		if err := repo.UpdateAttemptIntegrity(ctx, attempt); err != nil {
			return nil, err
		}
	}
	if err := repo.CreateIntegrityEvent(ctx, event); err != nil {
		return nil, err
	}
	if event.Level == model.IntegrityLevelViolation {
		*events = append(*events, pendingEvent{userID: attempt.UserID, payload: socket.AttemptEvent{
			Type: socket.EventAttemptViolation,
			Payload: socket.AttemptViolationPayload{
				AttemptID: attempt.ID, EventID: event.EventID,
				ViolationCount: attempt.ViolationCount, MaxViolations: dto.ToIntegrityState(attempt).MaxViolations,
			},
		}})
	}
	if exceededMaxViolations(attempt) {
		var err error
		if attempt, err = s.terminateAttempt(ctx, repo, attempt, model.IntegrityTerminationMaxViolations, "system", events); err != nil {
			return nil, err
		}
	}
	if !event.Stale {
		if err := s.queueIntegrityUpdate(ctx, repo, attempt, event, events); err != nil {
			return nil, err
		}
	}
	return attempt, nil
}

// recordAwayDuration ghi thời lượng rời màn (đo phía server) vào AWAY_END để giáo
// viên xem. AWAY_END chỉ là LOG: vi phạm đã được tính lúc nhận AWAY_START.
func recordAwayDuration(ctx context.Context, repo repository.Repository, attempt *model.Attempt, end *model.AttemptIntegrityEvent) error {
	start, err := repo.GetIntegrityEvent(ctx, attempt.ID, *end.StartEventID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if start.Type != model.IntegrityEventAwayStart {
		return nil
	}
	duration := int(end.ReceivedAt.Sub(start.ReceivedAt) / time.Millisecond)
	end.ServerDurationMs = &duration
	return nil
}

// integrityAck dựng phản hồi khi nhận một sự kiện giám sát: mức độ đã phân loại, số thứ tự vi
// phạm, có phải gửi trùng không, cùng trạng thái giám sát mới để client cập nhật cảnh báo.
func integrityAck(event *model.AttemptIntegrityEvent, attempt *model.Attempt, duplicate bool) *dto.IntegrityEventAckResponse {
	return &dto.IntegrityEventAckResponse{
		EventID:     event.EventID,
		Level:       string(event.Level),
		ViolationNo: event.ViolationNo,
		Duplicate:   duplicate,
		State:       dto.ToIntegrityState(attempt),
	}
}

// recordServerEvent ghi event do server sinh (eventId uuid server) và gom push cho giáo viên.
func (s *service) recordServerEvent(ctx context.Context, repo repository.Repository, attempt *model.Attempt, eventType model.IntegrityEventType, level model.IntegrityLevel, connectionID *string, detail *string, events *[]pendingEvent) error {
	event := &model.AttemptIntegrityEvent{
		AttemptID:    attempt.ID,
		EventID:      uuid.NewString(),
		SessionEpoch: attempt.SessionEpoch,
		ConnectionID: connectionID,
		Type:         eventType,
		Level:        level,
		Detail:       detail,
		ReceivedAt:   s.clock(),
	}
	if err := repo.CreateIntegrityEvent(ctx, event); err != nil {
		return err
	}
	return s.queueIntegrityUpdate(ctx, repo, attempt, event, events)
}

// queueIntegrityUpdate gom ATTEMPT_INTEGRITY_UPDATED cho giáo viên của lớp.
func (s *service) queueIntegrityUpdate(ctx context.Context, repo repository.Repository, attempt *model.Attempt, event *model.AttemptIntegrityEvent, events *[]pendingEvent) error {
	if attempt.ContextID == nil {
		return nil
	}
	assignment, err := repo.GetAssignmentForAttemptValidation(ctx, *attempt.ContextID)
	if err != nil {
		return err
	}
	state := dto.ToIntegrityState(attempt)
	payload := socket.AttemptIntegrityUpdatedPayload{
		ClassroomID:    assignment.ClassroomID,
		AssignmentID:   assignment.ID,
		StudentID:      attempt.UserID,
		AttemptID:      attempt.ID,
		ViolationCount: state.ViolationCount,
		MaxViolations:  state.MaxViolations,
		Status:         state.Status,
		Outcome:        state.Outcome,
	}
	if event != nil {
		record := &socket.AttemptIntegrityRecord{
			EventID:          event.EventID,
			Type:             string(event.Type),
			Level:            string(event.Level),
			ViolationNo:      event.ViolationNo,
			ServerDurationMs: event.ServerDurationMs,
			At:               event.ReceivedAt,
		}
		if event.Review != nil {
			review := string(*event.Review)
			record.Review = &review
		}
		payload.Event = record
		payload.Away = event.Type == model.IntegrityEventAwayStart && attempt.Status == model.AttemptStatusInProgress
	}
	*events = append(*events, pendingEvent{room: true, userID: assignment.Classroom.TeacherID, payload: socket.AttemptIntegrityUpdatedEvent{
		Type: socket.EventAttemptIntegrity, Payload: payload,
	}})
	return nil
}

// ReleaseAttemptConnection chạy khi một kết nối /ws/exams ngắt: bỏ quyền chủ phiên
// và ghi SessionLastSeenAt để tính thời gian chờ chiếm phiên.
func (s *service) ReleaseAttemptConnection(ctx context.Context, connectionID string) {
	attemptID, ok := s.integrity.releaseConnection(connectionID)
	if !ok {
		return
	}
	if err := s.repo.TouchAttemptSession(ctx, attemptID, s.clock()); err != nil {
		logger.Log.Error().Err(err).Uint("attempt_id", attemptID).Msg("[ERROR] Failed to record session disconnect")
	}
}

// requireSessionOwner: bài có giám sát chỉ nhận thao tác (đáp án, tiến trang nghe) từ kết
// nối đang giữ phiên; client claim phiên trước khi gửi đáp án.
func (s *service) requireSessionOwner(attempt *model.Attempt, connectionID string) error {
	if !attempt.Proctored || s.integrity.owner(attempt.ID) == connectionID {
		return nil
	}
	return platformsocket.NewHandlerError(socket.ErrorCodeSessionInUse, "[SOCKET] Attempt is open in another session")
}
