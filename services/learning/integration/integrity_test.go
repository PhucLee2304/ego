//go:build integration

package integration_test

import (
	"context"
	"testing"
	"time"

	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"

	"gorm.io/datatypes"
)

// Ràng buộc DB của giám sát (PLAN_ATTEMPT_INTEGRITY.md I1): chốt attempt đúng một lần,
// bài đình chỉ luôn 0 điểm, event idempotent theo eventId, một episode một vi phạm.
func TestIntegrityDatabaseInvariants(t *testing.T) {
	db := learningTestDB(t)
	ctx := context.Background()
	exam := seedTOEIC(t, db)
	created, httpErr := newAttemptService(db).CreateAttempt(ctx, "student-1", exam.ID, dto.CreateExamAttemptRequest{Mode: model.AttemptModeTest, ContextType: model.AttemptContextTypeStandalone})
	if httpErr != nil {
		t.Fatal(httpErr)
	}

	policy := datatypes.NewJSONType(model.IntegrityPolicySnapshot{MaxViolations: 2, ConsentVersion: "v1"})
	if err := db.Model(&model.Attempt{}).Where("id = ?", created.ID).Updates(map[string]any{"proctored": true, "integrity_policy": policy}).Error; err != nil {
		t.Fatal(err)
	}
	var stored model.Attempt
	if err := db.First(&stored, created.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.IntegrityPolicy == nil || stored.IntegrityPolicy.Data().MaxViolations != 2 || stored.IntegrityOutcome != model.IntegrityOutcomeNone {
		t.Fatalf("policy = %#v outcome = %s", stored.IntegrityPolicy, stored.IntegrityOutcome)
	}

	// Chốt hai lần: chỉ lần đầu đổi được dòng.
	repo := repository.NewRepository(db)
	now := time.Now()
	score := 0.0
	stored.Status, stored.SubmittedAt, stored.Score, stored.RawScore = model.AttemptStatusSubmitted, &now, &score, &score
	for i, want := range []bool{true, false} {
		ok, err := repo.FinalizeAttempt(ctx, &stored)
		if err != nil || ok != want {
			t.Fatalf("finalize #%d = %v, %v; want %v", i+1, ok, err, want)
		}
	}

	// Bài đình chỉ mà điểm khác 0 bị DB từ chối.
	if err := db.Model(&model.Attempt{}).Where("id = ?", created.ID).Updates(map[string]any{"integrity_outcome": model.IntegrityOutcomeTerminated, "score": 5}).Error; err == nil {
		t.Fatal("expected CHECK to reject terminated attempt with score 5")
	}
	if err := db.Model(&model.Attempt{}).Where("id = ?", created.ID).Updates(map[string]any{"integrity_outcome": model.IntegrityOutcomeTerminated, "score": 0}).Error; err != nil {
		t.Fatalf("terminated attempt with score 0 must pass: %v", err)
	}

	// MaxViolations phải ≥ 1.
	if err := db.Exec("INSERT INTO assignments (created_at, updated_at, title, exam_id, duration, opens_at, due_at, active, classroom_id, max_violations) VALUES (now(), now(), 'x', ?, 10, now(), now(), true, 0, 0)", exam.ID).Error; err == nil {
		t.Fatal("expected CHECK to reject max_violations = 0")
	}

	start := "start-1"
	event := func(eventID string, level model.IntegrityLevel) *model.AttemptIntegrityEvent {
		return &model.AttemptIntegrityEvent{AttemptID: created.ID, EventID: eventID, Type: model.IntegrityEventAwayEnd, Level: level, StartEventID: &start, ReceivedAt: now}
	}
	if err := db.Create(event("e1", model.IntegrityLevelViolation)).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(event("e1", model.IntegrityLevelLog)).Error; err == nil {
		t.Fatal("expected duplicate eventId to be rejected")
	}
	if err := db.Create(event("e2", model.IntegrityLevelViolation)).Error; err == nil {
		t.Fatal("expected second violation for the same episode to be rejected")
	}
	if err := db.Create(event("e3", model.IntegrityLevelLog)).Error; err != nil {
		t.Fatalf("non-violation event of the same episode must pass: %v", err)
	}
}
