//go:build integration

package integration_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"ego/platform/httpx"
	platformsocket "ego/platform/socket"
	"ego/services/learning/config"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"
	"ego/services/learning/internal/service"
	"ego/services/learning/internal/socket"
)

// Giám sát làm bài (PLAN_ATTEMPT_INTEGRITY.md mục 3): luồng tuần tự và đồng thời.

// pushType đọc type của một push WebSocket (map {type, payload} hoặc struct).
func pushType(payload any) string {
	switch event := payload.(type) {
	case map[string]any:
		value, _ := event["type"].(string)
		return value
	case socket.AttemptEvent:
		return event.Type
	case socket.AttemptIntegrityUpdatedEvent:
		return event.Type
	case socket.GradebookUpdatedEvent:
		return event.Type
	}
	return ""
}

// pushRecorder ghi mọi push của một hub.
type pushRecorder struct {
	mu       sync.Mutex
	payloads []any
}

func (r *pushRecorder) publish(_ string, payload any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.payloads = append(r.payloads, payload)
}

func (r *pushRecorder) count(eventType string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, payload := range r.payloads {
		if pushType(payload) == eventType {
			n++
		}
	}
	return n
}

// testClock là đồng hồ của service trong test, tiến bằng tay.
type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

type integrityFixture struct {
	*assignmentFixture
	clock        *testClock
	student      *pushRecorder
	teacher      *pushRecorder
	assignmentID uint
	examID       uint
}

func newIntegrityFixture(t *testing.T, maxViolations int) *integrityFixture {
	t.Helper()
	f := newAssignmentFixture(t)
	clock := &testClock{now: time.Now()}
	student, teacher := &pushRecorder{}, &pushRecorder{}
	f.svc = service.New(repository.NewRepository(f.db), &config.AppConfig{}, nil, testIdentityClient{}, nil, nil, teacher.publish, student.publish)
	service.SetClock(f.svc, clock.Now)
	schedule := f.schedule(f.classroom.ID)
	schedule.Proctored, schedule.MaxViolations = true, &maxViolations
	created, httpErr := f.svc.CreateAssignments(context.Background(), "teacher-1", dto.CreateAssignmentsRequest{
		Title: "Proctored quiz", Questions: []dto.CreateAssignmentQuestionRequest{assignmentQuestion("1 + 1"), assignmentQuestion("2 + 2")},
		Schedules: []dto.AssignmentScheduleRequest{schedule},
	})
	if httpErr != nil {
		t.Fatal(httpErr)
	}
	assignment := created.Assignments[0]
	if !assignment.Proctored || assignment.MaxViolations != maxViolations {
		t.Fatalf("assignment = %#v", assignment)
	}
	return &integrityFixture{assignmentFixture: f, clock: clock, student: student, teacher: teacher, assignmentID: assignment.ID, examID: assignment.ExamID}
}

func (f *integrityFixture) start(t *testing.T) *dto.AttemptResponse {
	t.Helper()
	req := assignmentAttemptRequest(f.assignmentID)
	req.IntegrityConsent = &dto.IntegrityConsentRequest{Version: config.DefaultIntegrityConsentVersion}
	attempt, httpErr := f.svc.CreateAttempt(context.Background(), "student-1", f.examID, req)
	if httpErr != nil {
		t.Fatal(httpErr)
	}
	return attempt
}

func (f *integrityFixture) claim(attemptID uint, conn, token string, takeover bool) (*dto.AttemptSessionClaimResponse, error) {
	return f.svc.ClaimAttemptSession(context.Background(), "student-1", conn, socket.AttemptSessionClaimPayload{AttemptID: attemptID, SessionToken: token, Takeover: takeover})
}

func (f *integrityFixture) mustClaim(t *testing.T, attemptID uint, conn, token string, takeover bool) *dto.AttemptSessionClaimResponse {
	t.Helper()
	claim, err := f.claim(attemptID, conn, token, takeover)
	if err != nil {
		t.Fatalf("claim %s: %v", conn, err)
	}
	return claim
}

func (f *integrityFixture) send(attemptID uint, conn string, epoch int, eventType, eventID string, startEventID *string, replayed bool) (*dto.IntegrityEventAckResponse, error) {
	return f.svc.RecordIntegrityEvent(context.Background(), "student-1", conn, socket.AttemptIntegrityEventPayload{
		AttemptID: attemptID, EventID: eventID, SessionEpoch: epoch, Type: eventType, StartEventID: startEventID, Replayed: replayed,
	})
}

// awayEpisode gửi một lần rời màn dài d; trả ACK của AWAY_START (vi phạm được tính
// lúc đó). AWAY_END chỉ gửi khi bài còn đang làm.
func (f *integrityFixture) awayEpisode(t *testing.T, attemptID uint, conn string, epoch int, id string, d time.Duration) *dto.IntegrityEventAckResponse {
	t.Helper()
	start := id + "-start"
	ack, err := f.send(attemptID, conn, epoch, "AWAY_START", start, nil, false)
	if err != nil {
		t.Fatalf("AWAY_START %s: %v", id, err)
	}
	f.clock.advance(d)
	if ack.State.Status == "ACTIVE" {
		if _, err := f.send(attemptID, conn, epoch, "AWAY_END", id+"-end", &start, false); err != nil {
			t.Fatalf("AWAY_END %s: %v", id, err)
		}
	}
	return ack
}

func (f *integrityFixture) attempt(t *testing.T, id uint) model.Attempt {
	t.Helper()
	var attempt model.Attempt
	if err := f.db.First(&attempt, id).Error; err != nil {
		t.Fatal(err)
	}
	return attempt
}

func requireCode(t *testing.T, err error, code platformsocket.ErrorCode) *platformsocket.HandlerError {
	t.Helper()
	var handlerErr *platformsocket.HandlerError
	if !errors.As(err, &handlerErr) || handlerErr.Code != code {
		t.Fatalf("error = %v, want %s", err, code)
	}
	return handlerErr
}

func TestIntegrityConsent(t *testing.T) {
	f := newIntegrityFixture(t, 2)
	ctx := context.Background()
	req := assignmentAttemptRequest(f.assignmentID)
	if _, httpErr := f.svc.CreateAttempt(ctx, "student-1", f.examID, req); httpErr == nil || httpErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("missing consent must be 400, got %v", httpErr)
	}
	req.IntegrityConsent = &dto.IntegrityConsentRequest{Version: "old"}
	if _, httpErr := f.svc.CreateAttempt(ctx, "student-1", f.examID, req); httpErr == nil || httpErr.StatusCode != http.StatusConflict {
		t.Fatalf("outdated consent must be 409, got %v", httpErr)
	}
	attempt := f.start(t)
	if !attempt.Proctored || attempt.MaxViolations == nil || *attempt.MaxViolations != 2 || *attempt.ViolationCount != 0 || *attempt.IntegrityOutcome != "NONE" {
		t.Fatalf("attempt = %#v", attempt)
	}
	stored := f.attempt(t, attempt.ID)
	if stored.IntegrityConsentAt == nil || stored.IntegrityConsentVersion == nil || *stored.IntegrityConsentVersion != config.DefaultIntegrityConsentVersion ||
		stored.IntegrityPolicy == nil || stored.IntegrityPolicy.Data().MaxViolations != 2 || stored.IntegrityPolicy.Data().TakeoverDelayMs != 30000 {
		t.Fatalf("consent/snapshot = %#v", stored)
	}
}

// Max = 2: lần 1, 2 vẫn làm; lần 3 đình chỉ, 0 điểm, RawScore giữ điểm chấm.
func TestIntegrityViolationsTerminate(t *testing.T) {
	f := newIntegrityFixture(t, 2)
	attempt := f.start(t)
	claim := f.mustClaim(t, attempt.ID, "c1", "", false)
	f.clock.advance(11 * time.Second)

	for i := 1; i <= 2; i++ {
		ack := f.awayEpisode(t, attempt.ID, "c1", claim.SessionEpoch, fmt.Sprintf("v%d", i), 5*time.Second)
		if ack.Level != "VIOLATION" || ack.ViolationNo == nil || *ack.ViolationNo != i || ack.State.Status != "ACTIVE" {
			t.Fatalf("violation %d ack = %#v", i, ack)
		}
	}
	ack := f.awayEpisode(t, attempt.ID, "c1", claim.SessionEpoch, "v3", 5*time.Second)
	if ack.Level != "VIOLATION" || ack.State.Status != "SUBMITTED" || ack.State.Outcome != "TERMINATED" {
		t.Fatalf("third violation ack = %#v", ack)
	}
	stored := f.attempt(t, attempt.ID)
	if stored.Score == nil || *stored.Score != 0 || stored.RawScore == nil || stored.TerminationReason == nil || *stored.TerminationReason != model.IntegrityTerminationMaxViolations {
		t.Fatalf("terminated attempt = %#v", stored)
	}
	var submission model.AssignmentSubmission
	f.db.Where("assignment_id = ? AND student_id = ?", f.assignmentID, "student-1").First(&submission)
	if submission.Score == nil || *submission.Score != 0 {
		t.Fatalf("submission = %#v", submission)
	}
	if f.student.count("ATTEMPT_VIOLATION") != 3 || f.student.count("ATTEMPT_TERMINATED") != 1 || f.student.count("ATTEMPT_SUBMITTED") != 1 {
		t.Fatalf("student pushes = %#v", f.student.payloads)
	}
	if f.teacher.count("ATTEMPT_INTEGRITY_UPDATED") == 0 {
		t.Fatal("teacher must receive ATTEMPT_INTEGRITY_UPDATED")
	}
	if _, err := f.send(attempt.ID, "c1", claim.SessionEpoch, "COPY", "after", nil, false); err == nil {
		t.Fatal("event after termination must be rejected")
	}

	// Gradebook phân biệt bài đình chỉ.
	gradebook, httpErr := f.svc.GetGradebook(context.Background(), "teacher-1", f.classroom.ID, f.assignmentID)
	if httpErr != nil {
		t.Fatal(httpErr)
	}
	row := gradebook.Submissions[0]
	if !row.Proctored || row.ViolationCount != 3 || row.PendingViolations != 3 || row.IntegrityOutcome == nil || *row.IntegrityOutcome != "TERMINATED" {
		t.Fatalf("gradebook row = %#v", row)
	}

	// Khôi phục: Score = RawScore, vẫn đã nộp; khôi phục lần nữa → 409.
	if _, httpErr := f.svc.ReinstateAttempt(context.Background(), "teacher-1", f.classroom.ID, f.assignmentID, "student-1", dto.ReinstateAttemptRequest{Note: "Mất mạng"}); httpErr != nil {
		t.Fatal(httpErr)
	}
	stored = f.attempt(t, attempt.ID)
	if stored.IntegrityOutcome != model.IntegrityOutcomeReinstated || stored.Status != model.AttemptStatusSubmitted || *stored.Score != *stored.RawScore {
		t.Fatalf("reinstated attempt = %#v", stored)
	}
	history, httpErr := f.svc.GetAttemptHistory(context.Background(), "student-1", attempt.ID)
	if httpErr != nil || history.IntegrityOutcome == nil || *history.IntegrityOutcome != "REINSTATED" {
		t.Fatalf("history = %#v err=%v", history, httpErr)
	}
	if _, httpErr := f.svc.ReinstateAttempt(context.Background(), "teacher-1", f.classroom.ID, f.assignmentID, "student-1", dto.ReinstateAttemptRequest{Note: "x"}); httpErr == nil || httpErr.StatusCode != http.StatusConflict {
		t.Fatalf("second reinstate must be 409, got %v", httpErr)
	}
}

// Rời màn là vi phạm ngay (mục 1.3) và idempotency (mục 1.5).
func TestIntegrityEpisodeClassification(t *testing.T) {
	f := newIntegrityFixture(t, 10)
	attempt := f.start(t)
	claim := f.mustClaim(t, attempt.ID, "c1", "", false)
	epoch := claim.SessionEpoch

	// Rời màn ngay sau claim, dù chỉ 1s → VIOLATION (không ân hạn, không ngưỡng).
	ack := f.awayEpisode(t, attempt.ID, "c1", epoch, "real", time.Second)
	if ack.Level != "VIOLATION" || *ack.ViolationNo != 1 {
		t.Fatalf("away = %#v", ack)
	}
	var end model.AttemptIntegrityEvent
	if err := f.db.Where("attempt_id = ? AND event_id = ?", attempt.ID, "real-end").First(&end).Error; err != nil ||
		end.Level != model.IntegrityLevelLog || end.ServerDurationMs == nil || *end.ServerDurationMs != 1000 {
		t.Fatalf("AWAY_END = %#v err=%v", end, err)
	}
	// Gửi lại cùng eventId → duplicate, không tính lại.
	dup, err := f.send(attempt.ID, "c1", epoch, "AWAY_START", "real-start", nil, false)
	if err != nil || !dup.Duplicate || dup.Level != "VIOLATION" || *dup.ViolationNo != 1 || dup.State.ViolationCount != 1 {
		t.Fatalf("duplicate = %#v err=%v", dup, err)
	}
	// Gửi bù (offline) → WARNING, không tính.
	if ack, err := f.send(attempt.ID, "c1", epoch, "AWAY_START", "offline-start", nil, true); err != nil || ack.Level != "WARNING" || ack.State.ViolationCount != 1 {
		t.Fatalf("replayed = %#v err=%v", ack, err)
	}
	start := "missing-start"
	if ack, err := f.send(attempt.ID, "c1", epoch, "AWAY_END", "orphan-end", &start, true); err != nil || ack.Level != "LOG" {
		t.Fatalf("orphan end = %#v err=%v", ack, err)
	}

	// F5: kết nối ngắt, claim lại bằng token, rời màn → VIOLATION.
	f.svc.ReleaseAttemptConnection(context.Background(), "c1")
	claim = f.mustClaim(t, attempt.ID, "c2", claim.SessionToken, false)
	if claim.SessionEpoch != epoch {
		t.Fatalf("resume must keep epoch: %d", claim.SessionEpoch)
	}
	if ack := f.awayEpisode(t, attempt.ID, "c2", epoch, "f5", time.Second); ack.Level != "VIOLATION" || *ack.ViolationNo != 2 {
		t.Fatalf("F5 then away = %#v", ack)
	}
	// Event từ kết nối cũ → stale, LOG.
	if ack := f.awayEpisode(t, attempt.ID, "c1", epoch, "stale", time.Minute); ack.Level != "LOG" || ack.State.ViolationCount != 2 {
		t.Fatalf("stale = %#v", ack)
	}
	if _, err := f.send(attempt.ID, "c2", epoch, "SCREENSHOT", "bad", nil, false); err == nil {
		t.Fatal("unknown type must be rejected")
	}

	events, _, httpErr := f.svc.GetIntegrityEvents(context.Background(), "teacher-1", f.classroom.ID, f.assignmentID, dto.IntegrityEventsQuery{Pending: true, PaginationQuery: pagination()})
	if httpErr != nil || len(events) != 2 {
		t.Fatalf("pending violations = %#v err=%v", events, httpErr)
	}
	if _, _, httpErr := f.svc.GetIntegrityEvents(context.Background(), "teacher-2", f.classroom.ID, f.assignmentID, dto.IntegrityEventsQuery{PaginationQuery: pagination()}); httpErr == nil || httpErr.StatusCode != http.StatusForbidden {
		t.Fatalf("other teacher must be 403, got %v", httpErr)
	}
}

// Claim phiên theo bảng mục 1.4.
func TestIntegritySessionClaims(t *testing.T) {
	f := newIntegrityFixture(t, 2)
	attempt := f.start(t)
	first := f.mustClaim(t, attempt.ID, "a", "", false)
	if first.SessionEpoch != 1 || first.SessionToken == "" {
		t.Fatalf("first claim = %#v", first)
	}
	// Tab nhân bản cầm token đúng nhưng chủ còn sống.
	_, err := f.claim(attempt.ID, "b", first.SessionToken, false)
	requireCode(t, err, socket.ErrorCodeSessionInUse)
	_, err = f.claim(attempt.ID, "b", "", true)
	requireCode(t, err, socket.ErrorCodeSessionInUse)

	// F5: chủ ngắt, token đúng → nhận, xoay token.
	f.svc.ReleaseAttemptConnection(context.Background(), "a")
	resumed := f.mustClaim(t, attempt.ID, "b", first.SessionToken, false)
	if resumed.SessionToken == first.SessionToken || resumed.SessionEpoch != 1 {
		t.Fatalf("resume = %#v", resumed)
	}

	// Thiết bị khác khi chủ vừa ngắt → chờ; quá 30s → phải xác nhận; xác nhận → epoch 2.
	f.svc.ReleaseAttemptConnection(context.Background(), "b")
	_, err = f.claim(attempt.ID, "c", first.SessionToken, false)
	handlerErr := requireCode(t, err, socket.ErrorCodeSessionRecovering)
	if details, ok := handlerErr.Details.(map[string]int64); !ok || details["retryAfterMs"] <= 0 {
		t.Fatalf("details = %#v", handlerErr.Details)
	}
	f.clock.advance(31 * time.Second)
	_, err = f.claim(attempt.ID, "c", "", false)
	requireCode(t, err, socket.ErrorCodeSessionTakeoverRequired)
	takeover := f.mustClaim(t, attempt.ID, "c", "", true)
	if takeover.SessionEpoch != 2 {
		t.Fatalf("takeover = %#v", takeover)
	}
	if f.student.count("ATTEMPT_SESSION_REVOKED") != 1 {
		t.Fatalf("takeover must revoke the old epoch: %#v", f.student.payloads)
	}
	// Phiên cũ gửi event → stale.
	if ack, err := f.send(attempt.ID, "b", 1, "COPY", "old", nil, false); err != nil || ack.Level != "LOG" {
		t.Fatalf("old session event = %#v err=%v", ack, err)
	}
	var stale model.AttemptIntegrityEvent
	f.db.Where("event_id = ?", "old").First(&stale)
	if !stale.Stale {
		t.Fatalf("old session event must be stale: %#v", stale)
	}

	// Giáo viên giải phóng phiên → claim mới nhận ngay, epoch 3.
	if _, httpErr := f.svc.ReleaseAttemptSession(context.Background(), "teacher-1", f.classroom.ID, f.assignmentID, "student-1"); httpErr != nil {
		t.Fatal(httpErr)
	}
	if released := f.mustClaim(t, attempt.ID, "d", "", false); released.SessionEpoch != 3 {
		t.Fatalf("claim after release = %#v", released)
	}
	var conflicts, takeovers int64
	f.db.Model(&model.AttemptIntegrityEvent{}).Where("type = ?", model.IntegrityEventSessionConflict).Count(&conflicts)
	f.db.Model(&model.AttemptIntegrityEvent{}).Where("type = ?", model.IntegrityEventSessionTakeover).Count(&takeovers)
	if conflicts != 2 || takeovers != 1 {
		t.Fatalf("conflicts=%d takeovers=%d", conflicts, takeovers)
	}
}

// Review: CONTINUE lúc đang làm trừ count 1 (lặp lại không trừ nữa); lặp lại cùng action 200, đổi action 409; Cấm → 0 điểm.
func TestIntegrityReviewAndBan(t *testing.T) {
	f := newIntegrityFixture(t, 5)
	ctx := context.Background()
	attempt := f.start(t)
	claim := f.mustClaim(t, attempt.ID, "c1", "", false)
	f.clock.advance(11 * time.Second)
	f.awayEpisode(t, attempt.ID, "c1", claim.SessionEpoch, "v1", 5*time.Second)

	review := func(eventID, action string) (*dto.AttemptIntegrityResponse, error) {
		resp, httpErr := f.svc.ReviewIntegrityEvent(ctx, "teacher-1", f.classroom.ID, f.assignmentID, eventID, dto.ReviewIntegrityEventRequest{Action: action})
		if httpErr != nil {
			return nil, httpErr
		}
		return resp, nil
	}
	resp, err := review("v1-start", "CONTINUE")
	if err != nil || resp.ViolationCount != 0 || resp.Status != "ACTIVE" {
		t.Fatalf("continue = %#v err=%v", resp, err)
	}
	if resp, err := review("v1-start", "CONTINUE"); err != nil || resp.ViolationCount != 0 {
		t.Fatalf("same action again must be 200 without decrement: %#v %v", resp, err)
	}
	if _, httpErr := f.svc.ReviewIntegrityEvent(ctx, "teacher-1", f.classroom.ID, f.assignmentID, "v1-start", dto.ReviewIntegrityEventRequest{Action: "BAN"}); httpErr == nil || httpErr.StatusCode != http.StatusConflict {
		t.Fatalf("changing action must be 409, got %v", httpErr)
	}
	if _, httpErr := f.svc.ReviewIntegrityEvent(ctx, "teacher-1", f.classroom.ID, f.assignmentID, "v1-end", dto.ReviewIntegrityEventRequest{Action: "CONTINUE"}); httpErr == nil || httpErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("reviewing a non-violation must be 400, got %v", httpErr)
	}

	f.awayEpisode(t, attempt.ID, "c1", claim.SessionEpoch, "v2", 5*time.Second)
	resp, err = review("v2-start", "BAN")
	if err != nil || resp.Status != "SUBMITTED" || resp.IntegrityOutcome != "TERMINATED" || resp.TerminationReason == nil || *resp.TerminationReason != "TEACHER_BANNED" || resp.Score == nil || *resp.Score != 0 {
		t.Fatalf("ban = %#v err=%v", resp, err)
	}
	if _, httpErr := f.svc.BanAttempt(ctx, "teacher-1", f.classroom.ID, f.assignmentID, "student-1", dto.BanAttemptRequest{}); httpErr == nil || httpErr.StatusCode != http.StatusConflict {
		t.Fatalf("ban after submit must be 409, got %v", httpErr)
	}
}

// Attempt không giám sát: không claim, event bị bỏ qua. Reset đẩy ATTEMPT_RESET.
func TestIntegrityUnproctoredAndReset(t *testing.T) {
	f := newIntegrityFixture(t, 2)
	ctx := context.Background()
	plain := f.create(t, f.classroom.ID).Assignments[0]
	attempt, httpErr := f.svc.CreateAttempt(ctx, "student-1", plain.ExamID, assignmentAttemptRequest(plain.ID))
	if httpErr != nil {
		t.Fatal(httpErr)
	}
	_, err := f.claim(attempt.ID, "c1", "", false)
	requireCode(t, err, socket.ErrorCodeAttemptNotProctored)
	if ack, err := f.send(attempt.ID, "c1", 0, "COPY", "e1", nil, false); err != nil || !ack.Ignored {
		t.Fatalf("unproctored event = %#v err=%v", ack, err)
	}
	if _, httpErr := f.svc.SubmitAttempt(ctx, "student-1", attempt.ID, dto.SubmitAttemptRequest{}); httpErr != nil {
		t.Fatal(httpErr)
	}

	proctored := f.start(t)
	claim := f.mustClaim(t, proctored.ID, "c1", "", false)
	if httpErr := f.svc.ResetAssignmentAttempt(ctx, "teacher-1", f.classroom.ID, f.assignmentID, "student-1"); httpErr != nil {
		t.Fatal(httpErr)
	}
	if f.student.count("ATTEMPT_RESET") != 1 {
		t.Fatalf("reset must push ATTEMPT_RESET: %#v", f.student.payloads)
	}
	_, err = f.send(proctored.ID, "c1", claim.SessionEpoch, "COPY", "after-reset", nil, false)
	requireCode(t, err, socket.ErrorCodeAttemptNotFound)
}

func pagination() httpx.PaginationQuery {
	return httpx.PaginationQuery{Page: 1, PageSize: 50}
}

// Chấm lại sau khi đình chỉ: RawScore đổi, Score vẫn 0.
func TestIntegrityRegradeKeepsTerminatedScore(t *testing.T) {
	f := newIntegrityFixture(t, 1)
	ctx := context.Background()
	attempt := f.start(t)
	if _, httpErr := f.svc.BanAttempt(ctx, "teacher-1", f.classroom.ID, f.assignmentID, "student-1", dto.BanAttemptRequest{}); httpErr != nil {
		t.Fatal(httpErr)
	}
	editable, httpErr := f.svc.GetEditableAssignment(ctx, "teacher-1", f.classroom.ID, f.assignmentID)
	if httpErr != nil {
		t.Fatal(httpErr)
	}
	questions := make([]dto.UpdateAssignmentQuestionRequest, len(editable.Questions))
	for i, q := range editable.Questions {
		explanation := ""
		if q.Explanation != nil {
			explanation = *q.Explanation
		}
		questions[i] = dto.UpdateAssignmentQuestionRequest{ID: q.ID, Content: q.Content, Explanation: &explanation, Options: q.Options, OptionIDs: q.OptionIDs, CorrectOptionIndex: 1, ExcludedFromScore: true}
	}
	if _, httpErr := f.svc.UpdateAssignmentContent(ctx, "teacher-1", f.classroom.ID, f.assignmentID, dto.UpdateAssignmentContentRequest{
		Title: editable.Title, Description: editable.Description, Questions: questions, Confirm: true, Reason: "Fix key",
	}); httpErr != nil {
		t.Fatal(httpErr)
	}
	stored := f.attempt(t, attempt.ID)
	if stored.IntegrityOutcome != model.IntegrityOutcomeTerminated || stored.Score == nil || *stored.Score != 0 || stored.RawScore == nil {
		t.Fatalf("regraded terminated attempt = %#v", stored)
	}
}

// runConcurrently chạy các hàm cùng lúc sau một barrier.
func runConcurrently(fns ...func()) {
	var ready, done sync.WaitGroup
	gate := make(chan struct{})
	ready.Add(len(fns))
	done.Add(len(fns))
	for _, fn := range fns {
		go func(fn func()) {
			defer done.Done()
			ready.Done()
			<-gate
			fn()
		}(fn)
	}
	ready.Wait()
	close(gate)
	done.Wait()
}

const concurrencyRounds = 20

// resetAndStart đưa học sinh về attempt mới đã claim, qua ân hạn.
func (f *integrityFixture) resetAndStart(t *testing.T, round int) (*dto.AttemptResponse, *dto.AttemptSessionClaimResponse, string) {
	t.Helper()
	if round > 0 {
		if httpErr := f.svc.ResetAssignmentAttempt(context.Background(), "teacher-1", f.classroom.ID, f.assignmentID, "student-1"); httpErr != nil {
			t.Fatal(httpErr)
		}
	}
	attempt := f.start(t)
	conn := fmt.Sprintf("conn-%d", round)
	claim := f.mustClaim(t, attempt.ID, conn, "", false)
	f.clock.advance(11 * time.Second)
	return attempt, claim, conn
}

func TestIntegrityConcurrentDuplicateAndDistinctEvents(t *testing.T) {
	f := newIntegrityFixture(t, 10)
	for round := 0; round < concurrencyRounds; round++ {
		attempt, claim, conn := f.resetAndStart(t, round)
		// Cùng eventId gửi song song: chỉ một lần không phải duplicate. Đồng thời một
		// lần rời màn khác: count tăng đúng 2.
		var mu sync.Mutex
		fresh := 0
		var fns []func()
		for i := 0; i < 8; i++ {
			fns = append(fns, func() {
				ack, err := f.send(attempt.ID, conn, claim.SessionEpoch, "AWAY_START", "a-start", nil, false)
				if err != nil {
					t.Errorf("round %d: %v", round, err)
					return
				}
				mu.Lock()
				defer mu.Unlock()
				if !ack.Duplicate {
					fresh++
				}
			})
		}
		fns = append(fns, func() {
			if _, err := f.send(attempt.ID, conn, claim.SessionEpoch, "AWAY_START", "b-start", nil, false); err != nil {
				t.Errorf("round %d: %v", round, err)
			}
		})
		runConcurrently(fns...)
		stored := f.attempt(t, attempt.ID)
		if fresh != 1 || stored.ViolationCount != 2 {
			t.Fatalf("round %d: fresh=%d violationCount=%d", round, fresh, stored.ViolationCount)
		}
	}
}

// requireConsistentFinish kiểm bất biến sau khi hai luồng chốt bài đua nhau.
func (f *integrityFixture) requireConsistentFinish(t *testing.T, round int, attemptID uint) {
	t.Helper()
	stored := f.attempt(t, attemptID)
	if stored.Status != model.AttemptStatusSubmitted || stored.Score == nil {
		t.Fatalf("round %d: attempt = %#v", round, stored)
	}
	if stored.IntegrityOutcome == model.IntegrityOutcomeTerminated && *stored.Score != 0 {
		t.Fatalf("round %d: terminated attempt with score %v", round, *stored.Score)
	}
	var submission model.AssignmentSubmission
	f.db.Where("assignment_id = ? AND student_id = ?", f.assignmentID, "student-1").First(&submission)
	if submission.Score == nil || *submission.Score != *stored.Score {
		t.Fatalf("round %d: submission %v vs attempt %v", round, submission.Score, *stored.Score)
	}
	var outbox int64
	f.db.Model(&model.NotificationOutbox{}).Where("event_key = ?", fmt.Sprintf("attempt:%d:submitted", attemptID)).Count(&outbox)
	if outbox != 1 {
		t.Fatalf("round %d: outbox rows = %d", round, outbox)
	}
}

func TestIntegrityConcurrentSubmitAndBan(t *testing.T) {
	f := newIntegrityFixture(t, 1)
	ctx := context.Background()
	for round := 0; round < concurrencyRounds; round++ {
		attempt, _, _ := f.resetAndStart(t, round)
		var banErr, submitErr error
		runConcurrently(
			func() {
				if _, httpErr := f.svc.BanAttempt(ctx, "teacher-1", f.classroom.ID, f.assignmentID, "student-1", dto.BanAttemptRequest{}); httpErr != nil {
					banErr = httpErr
				}
			},
			func() {
				if _, httpErr := f.svc.SubmitAttempt(ctx, "student-1", attempt.ID, dto.SubmitAttemptRequest{}); httpErr != nil {
					submitErr = httpErr
				}
			},
		)
		if submitErr != nil {
			t.Fatalf("round %d: submit must return the stored result: %v", round, submitErr)
		}
		// Bên thắng quyết định kết quả: Cấm thắng → đình chỉ; Cấm thua → 409, bài nộp thường.
		want := model.IntegrityOutcomeTerminated
		if banErr != nil {
			want = model.IntegrityOutcomeNone
		}
		if stored := f.attempt(t, attempt.ID); stored.IntegrityOutcome != want {
			t.Fatalf("round %d: outcome %s, want %s (ban err %v)", round, stored.IntegrityOutcome, want, banErr)
		}
		f.requireConsistentFinish(t, round, attempt.ID)
	}
}

func TestIntegrityConcurrentThresholdAndSubmit(t *testing.T) {
	f := newIntegrityFixture(t, 1)
	ctx := context.Background()
	for round := 0; round < concurrencyRounds; round++ {
		attempt, claim, conn := f.resetAndStart(t, round)
		f.awayEpisode(t, attempt.ID, conn, claim.SessionEpoch, "first", 5*time.Second)
		runConcurrently(
			func() { _, _ = f.send(attempt.ID, conn, claim.SessionEpoch, "AWAY_START", "second-start", nil, false) },
			func() { _, _ = f.svc.SubmitAttempt(ctx, "student-1", attempt.ID, dto.SubmitAttemptRequest{}) },
		)
		f.requireConsistentFinish(t, round, attempt.ID)
	}
}

func TestIntegrityConcurrentAutoSubmit(t *testing.T) {
	f := newIntegrityFixture(t, 2)
	ctx := context.Background()
	for round := 0; round < concurrencyRounds; round++ {
		attempt, _, _ := f.resetAndStart(t, round)
		if err := f.db.Model(&model.Attempt{}).Where("id = ?", attempt.ID).Update("expires_at", time.Now().Add(-time.Second)).Error; err != nil {
			t.Fatal(err)
		}
		runConcurrently(
			func() { _, _ = f.svc.AutoSubmitExpiredAttempts(ctx, 10) },
			func() { _, _ = f.svc.AutoSubmitExpiredAttempts(ctx, 10) },
		)
		f.requireConsistentFinish(t, round, attempt.ID)
	}
}
