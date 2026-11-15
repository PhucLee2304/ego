//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/service"
	"ego/services/learning/internal/socket"
)

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
		attempt, claim, _ := f.resetAndStart(t, round)
		var banErr, submitErr error
		runConcurrently(
			func() {
				if _, httpErr := f.svc.BanAttempt(ctx, "teacher-1", f.classroom.ID, f.assignmentID, "student-1", dto.BanAttemptRequest{}); httpErr != nil {
					banErr = httpErr
				}
			},
			func() {
				if _, httpErr := f.svc.SubmitAttempt(ctx, "student-1", attempt.ID, dto.SubmitAttemptRequest{SessionToken: claim.SessionToken}); httpErr != nil {
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
			func() {
				_, _ = f.svc.SubmitAttempt(ctx, "student-1", attempt.ID, dto.SubmitAttemptRequest{SessionToken: claim.SessionToken})
			},
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

func TestIntegritySubmitRequiresSessionToken(t *testing.T) {
	f := newIntegrityFixture(t, 2)
	ctx := context.Background()
	attempt, claim, _ := f.resetAndStart(t, 0)
	for _, token := range []string{"", "wrong-token"} {
		if _, httpErr := f.svc.SubmitAttempt(ctx, "student-1", attempt.ID, dto.SubmitAttemptRequest{SessionToken: token}); httpErr == nil || httpErr.Message != service.ErrAttemptSessionInUse.Message {
			t.Fatalf("token %q: submit = %v, want session in use", token, httpErr)
		}
	}
	if _, httpErr := f.svc.SubmitAttempt(ctx, "student-1", attempt.ID, dto.SubmitAttemptRequest{SessionToken: claim.SessionToken}); httpErr != nil {
		t.Fatal(httpErr)
	}
}

func TestIntegrityAnswerRequiresSessionOwner(t *testing.T) {
	f := newIntegrityFixture(t, 2)
	ctx := context.Background()
	attempt, _, conn := f.resetAndStart(t, 0)
	question := attempt.Sections[0].Questions[0]
	option := question.Options[0].ID

	_, err := f.svc.UpdateAttemptAnswer(ctx, "student-1", "other-conn", attempt.ID, question.ID, &option)
	requireCode(t, err, socket.ErrorCodeSessionInUse)
	var saved int64
	f.db.Model(&model.AttemptAnswer{}).Where("attempt_id = ?", attempt.ID).Count(&saved)
	if saved != 0 {
		t.Fatalf("non-owner answer was saved (%d rows)", saved)
	}
	if _, err := f.svc.UpdateAttemptAnswer(ctx, "student-1", conn, attempt.ID, question.ID, &option); err != nil {
		t.Fatalf("owner answer: %v", err)
	}

	f.clock.advance(46 * time.Minute)
	_, err = f.svc.UpdateAttemptAnswer(ctx, "student-1", "other-conn", attempt.ID, question.ID, &option)
	requireCode(t, err, socket.ErrorCodeAttemptExpired)
}
