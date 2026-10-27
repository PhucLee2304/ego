package dto

import (
	"time"

	"ego/platform/httpx"
	"ego/services/learning/internal/model"
)

// Giám sát làm bài (PLAN_ATTEMPT_INTEGRITY.md mục 1.7–1.9).

// IntegrityConsentRequest là xác nhận đồng ý quy chế, gửi kèm khi tạo attempt có giám sát.
type IntegrityConsentRequest struct {
	Version string `json:"version" validate:"required,notblank"`
}

// IntegrityState là trạng thái giám sát học sinh thấy (ACK của WS và push).
type IntegrityState struct {
	Proctored      bool   `json:"proctored"`
	Status         string `json:"status"`
	ViolationCount int    `json:"violationCount"`
	MaxViolations  int    `json:"maxViolations"`
	Outcome        string `json:"outcome"`
}

func ToIntegrityState(attempt *model.Attempt) IntegrityState {
	state := IntegrityState{
		Proctored:      attempt.Proctored,
		Status:         string(attempt.Status),
		ViolationCount: attempt.ViolationCount,
		Outcome:        string(attempt.IntegrityOutcome),
	}
	if attempt.IntegrityPolicy != nil {
		state.MaxViolations = attempt.IntegrityPolicy.Data().MaxViolations
	}
	return state
}

type AttemptSessionClaimResponse struct {
	SessionToken string         `json:"sessionToken"`
	SessionEpoch int            `json:"sessionEpoch"`
	GraceUntil   *time.Time     `json:"graceUntil,omitempty"`
	State        IntegrityState `json:"state"`
}

type IntegrityEventAckResponse struct {
	EventID     string         `json:"eventId"`
	Level       string         `json:"level,omitempty"`
	ViolationNo *int           `json:"violationNo,omitempty"`
	Duplicate   bool           `json:"duplicate"`
	Ignored     bool           `json:"ignored,omitempty"`
	State       IntegrityState `json:"state"`
}

type IntegrityEventsQuery struct {
	httpx.PaginationQuery
	StudentID string `schema:"studentId"`
	Pending   bool   `schema:"pending"`
	Level     string `schema:"level"`
}

type ReviewIntegrityEventRequest struct {
	Action string `json:"action" validate:"required,oneof=CONTINUE BAN"`
}

type BanAttemptRequest struct {
	Note *string `json:"note,omitempty"`
}

type ReinstateAttemptRequest struct {
	Note string `json:"note" validate:"required,notblank"`
}

// AttemptIntegrityEventResponse là một dòng trong log giám sát của giáo viên.
type AttemptIntegrityEventResponse struct {
	EventID          string     `json:"eventId"`
	AttemptID        uint       `json:"attemptId"`
	StudentID        string     `json:"studentId"`
	Type             string     `json:"type"`
	Level            string     `json:"level"`
	ViolationNo      *int       `json:"violationNo,omitempty"`
	StartEventID     *string    `json:"startEventId,omitempty"`
	ServerDurationMs *int       `json:"serverDurationMs,omitempty"`
	ClientDurationMs *int       `json:"clientDurationMs,omitempty"`
	Replayed         bool       `json:"replayed"`
	Stale            bool       `json:"stale"`
	Detail           *string    `json:"detail,omitempty"`
	ClientAt         *time.Time `json:"clientAt,omitempty"`
	ReceivedAt       time.Time  `json:"receivedAt"`
	Review           *string    `json:"review,omitempty"`
	ReviewedAt       *time.Time `json:"reviewedAt,omitempty"`
}

func ToAttemptIntegrityEventResponse(event *model.AttemptIntegrityEvent, studentID string) *AttemptIntegrityEventResponse {
	response := &AttemptIntegrityEventResponse{
		EventID:          event.EventID,
		AttemptID:        event.AttemptID,
		StudentID:        studentID,
		Type:             string(event.Type),
		Level:            string(event.Level),
		ViolationNo:      event.ViolationNo,
		StartEventID:     event.StartEventID,
		ServerDurationMs: event.ServerDurationMs,
		ClientDurationMs: event.ClientDurationMs,
		Replayed:         event.Replayed,
		Stale:            event.Stale,
		Detail:           event.Detail,
		ClientAt:         event.ClientAt,
		ReceivedAt:       event.ReceivedAt,
		ReviewedAt:       event.ReviewedAt,
	}
	if event.Review != nil {
		review := string(*event.Review)
		response.Review = &review
	}
	return response
}

// AttemptIntegrityResponse là trạng thái giám sát của một attempt sau thao tác của giáo viên.
type AttemptIntegrityResponse struct {
	AttemptID         uint     `json:"attemptId"`
	StudentID         string   `json:"studentId"`
	Status            string   `json:"status"`
	Proctored         bool     `json:"proctored"`
	ViolationCount    int      `json:"violationCount"`
	MaxViolations     int      `json:"maxViolations"`
	IntegrityOutcome  string   `json:"integrityOutcome"`
	TerminationReason *string  `json:"terminationReason,omitempty"`
	Score             *float64 `json:"score,omitempty"`
	RawScore          *float64 `json:"rawScore,omitempty"`
}

func ToAttemptIntegrityResponse(attempt *model.Attempt) *AttemptIntegrityResponse {
	state := ToIntegrityState(attempt)
	return &AttemptIntegrityResponse{
		AttemptID:         attempt.ID,
		StudentID:         attempt.UserID,
		Status:            state.Status,
		Proctored:         state.Proctored,
		ViolationCount:    state.ViolationCount,
		MaxViolations:     state.MaxViolations,
		IntegrityOutcome:  state.Outcome,
		TerminationReason: terminationReason(attempt.TerminationReason),
		Score:             attempt.Score,
		RawScore:          attempt.RawScore,
	}
}

func terminationReason(reason *model.IntegrityTerminationReason) *string {
	if reason == nil {
		return nil
	}
	value := string(*reason)
	return &value
}
