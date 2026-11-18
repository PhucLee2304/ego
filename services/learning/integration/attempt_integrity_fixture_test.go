//go:build integration

package integration_test

import (
	"context"
	"errors"
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

func pagination() httpx.PaginationQuery {
	return httpx.PaginationQuery{Page: 1, PageSize: 50}
}
