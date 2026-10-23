//go:build integration

package integration_test

import (
	"context"
	"errors"
	"testing"

	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"

	"gorm.io/gorm"
)

func privateExamQuestion(content string, correct int) model.Question {
	keys := []model.OptionKey{model.OptionA, model.OptionB, model.OptionC, model.OptionD}
	options := make([]model.Option, len(keys))
	for i, key := range keys {
		text := content + "-" + string(key)
		options[i] = model.Option{Key: key, Content: &text, IsCorrect: i == correct, Order: i + 1}
	}
	return model.Question{Content: content, Order: 1, Options: options}
}

// Đề private chỉ đọc được qua GetPrivateAssignmentExam; sửa đề thay toàn bộ câu
// hỏi và option cũ trong cùng transaction.
func TestPrivateExamRepository(t *testing.T) {
	db := learningTestDB(t)
	ctx := context.Background()
	repo := repository.NewRepository(db)

	exam := &model.Exam{
		Title: "Quiz", Type: model.ExamTypeAssignment, IsPublic: false,
		Sections: []model.Section{{Code: model.SectionCodeFull, Title: "Assignment", Order: 1, Questions: []model.Question{privateExamQuestion("old", 0)}}},
	}
	if err := repo.CreateExam(ctx, exam); err != nil {
		t.Fatal(err)
	}
	public := &model.Exam{Title: "THPT", Type: model.ExamTypeTHPT, IsPublic: true}
	if err := repo.CreateExam(ctx, public); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetPrivateAssignmentExam(ctx, public.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("public exam must not be read as private, err=%v", err)
	}

	loaded, err := repo.GetPrivateAssignmentExam(ctx, exam.ID)
	if err != nil || len(loaded.Sections) != 1 || len(loaded.Sections[0].Questions) != 1 || len(loaded.Sections[0].Questions[0].Options) != 4 {
		t.Fatalf("loaded = %#v err=%v", loaded, err)
	}

	err = repo.Transaction(ctx, func(tx repository.Repository) error {
		loaded.Title = "Quiz v2"
		replacement := []model.Question{privateExamQuestion("new-1", 2), privateExamQuestion("new-2", 3)}
		replacement[1].Order = 2
		if err := tx.ReplacePrivateExamQuestions(ctx, loaded, replacement); err != nil {
			return err
		}
		return tx.CreateExamAudit(ctx, &model.ExamAudit{ExamID: exam.ID, ActorID: "teacher-1", Before: []byte("{}"), After: []byte("{}"), ScoreChanges: []byte("[]")})
	})
	if err != nil {
		t.Fatal(err)
	}

	updated, err := repo.GetPrivateAssignmentExam(ctx, exam.ID)
	if err != nil {
		t.Fatal(err)
	}
	questions := updated.Sections[0].Questions
	if updated.Title != "Quiz v2" || len(questions) != 2 || questions[0].Content != "new-1" || !questions[1].Options[3].IsCorrect {
		t.Fatalf("updated = %#v", updated)
	}
	var optionCount, auditCount int64
	db.Model(&model.Option{}).Count(&optionCount)
	db.Model(&model.ExamAudit{}).Where("exam_id = ?", exam.ID).Count(&auditCount)
	if optionCount != 8 || auditCount != 1 {
		t.Fatalf("options = %d (old ones must be deleted), audits = %d", optionCount, auditCount)
	}
}
