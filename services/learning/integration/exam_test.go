//go:build integration

package integration_test

import (
	"context"
	"net/http"
	"testing"

	"ego/services/learning/config"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"
	"ego/services/learning/internal/service"
)

// Catalog đề chỉ trả đề public; đề private của lớp không lộ qua /exams.
func TestExamCatalogHidesPrivateExams(t *testing.T) {
	db := learningTestDB(t)
	ctx := context.Background()
	public := &model.Exam{Title: "THPT 2024", Type: model.ExamTypeTHPT, IsPublic: true}
	private := &model.Exam{Title: "Class quiz", Type: model.ExamTypeTHPT, IsPublic: false}
	toeic := &model.Exam{Title: "TOEIC 1", Type: model.ExamTypeTOEIC, IsPublic: true}
	for _, exam := range []*model.Exam{public, private, toeic} {
		if err := db.Create(exam).Error; err != nil {
			t.Fatal(err)
		}
	}
	svc := service.New(repository.NewRepository(db), &config.AppConfig{}, nil, testIdentityClient{}, nil, nil, nil, nil)

	query := dto.GetExamsQuery{}
	query.Normalize()
	exams, total, err := svc.GetList(ctx, query)
	if err != nil || total != 1 || len(exams) != 1 || exams[0].ID != public.ID {
		t.Fatalf("THPT catalog = %#v total=%d err=%v", exams, total, err)
	}

	if got, err := svc.GetByID(ctx, public.ID); err != nil || got.ID != public.ID {
		t.Fatalf("get public exam = %#v err=%v", got, err)
	}
	if _, err := svc.GetByID(ctx, private.ID); err == nil || err.StatusCode != http.StatusNotFound {
		t.Fatalf("private exam must be 404, got %v", err)
	}
}
