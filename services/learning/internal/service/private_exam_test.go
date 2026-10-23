package service

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"ego/platform/httpx"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"
)

type fakeExamRepository struct {
	repository.Repository
	exam     *model.Exam
	replaced []model.Question
	audits   []*model.ExamAudit
}

func (f *fakeExamRepository) Transaction(_ context.Context, fn func(repository.Repository) error) error {
	return fn(f)
}

func (f *fakeExamRepository) CreateExam(_ context.Context, exam *model.Exam) error {
	f.exam = exam
	return nil
}

func (f *fakeExamRepository) GetPrivateAssignmentExam(context.Context, uint) (*model.Exam, error) {
	return f.exam, nil
}

func (f *fakeExamRepository) ReplacePrivateExamQuestions(_ context.Context, _ *model.Exam, questions []model.Question) error {
	f.replaced = questions
	return nil
}

func (f *fakeExamRepository) CreateExamAudit(_ context.Context, audit *model.ExamAudit) error {
	f.audits = append(f.audits, audit)
	return nil
}

func validPrivateQuestion() dto.CreatePrivateExamQuestionRequest {
	return dto.CreatePrivateExamQuestionRequest{Content: " 1 + 1 = ? ", Options: []string{"1", " 2 ", "3", "4"}, CorrectOptionIndex: 1}
}

func TestCreatePrivateExamRejectsEmptyQuestions(t *testing.T) {
	svc := &service{}
	if _, err := svc.createPrivateExam(context.Background(), dto.CreatePrivateExamRequest{Title: "Empty exam"}); err == nil {
		t.Fatal("expected an exam without questions to be rejected")
	}
}

func TestCreatePrivateExamValidation(t *testing.T) {
	blankOption := validPrivateQuestion()
	blankOption.Options = []string{"1", " ", "3", "4"}
	threeOptions := validPrivateQuestion()
	threeOptions.Options = []string{"1", "2", "3"}
	badCorrect := validPrivateQuestion()
	badCorrect.CorrectOptionIndex = 4

	tests := []struct {
		name    string
		req     dto.CreatePrivateExamRequest
		message string
	}{
		{"blank title", dto.CreatePrivateExamRequest{Title: "  ", Questions: []dto.CreatePrivateExamQuestionRequest{validPrivateQuestion()}}, "[ERROR] Exam title is required"},
		{"no questions", dto.CreatePrivateExamRequest{Title: "Quiz"}, "[ERROR] Exam must contain at least one question"},
		{"three options", dto.CreatePrivateExamRequest{Title: "Quiz", Questions: []dto.CreatePrivateExamQuestionRequest{threeOptions}}, "[ERROR] Each question must have content, exactly four options, and one valid correct option"},
		{"correct index out of range", dto.CreatePrivateExamRequest{Title: "Quiz", Questions: []dto.CreatePrivateExamQuestionRequest{badCorrect}}, "[ERROR] Each question must have content, exactly four options, and one valid correct option"},
		{"blank option", dto.CreatePrivateExamRequest{Title: "Quiz", Questions: []dto.CreatePrivateExamQuestionRequest{blankOption}}, "[ERROR] Question options cannot be blank"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeExamRepository{}
			_, err := (&service{repo: repo}).createPrivateExam(context.Background(), tt.req)
			var httpErr *httpx.HTTPError
			if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusBadRequest || httpErr.Message != tt.message {
				t.Fatalf("err = %v, want 400 %q", err, tt.message)
			}
			if repo.exam != nil {
				t.Fatal("invalid exam must not be stored")
			}
		})
	}
}

func TestCreatePrivateExamBuildsAssignmentExam(t *testing.T) {
	repo := &fakeExamRepository{}
	exam, err := (&service{repo: repo}).createPrivateExam(context.Background(), dto.CreatePrivateExamRequest{
		Title:     " Quiz ",
		Questions: []dto.CreatePrivateExamQuestionRequest{validPrivateQuestion()},
	})
	if err != nil {
		t.Fatal(err)
	}
	if exam != repo.exam || exam.Title != "Quiz" || exam.IsPublic || exam.Type != model.ExamTypeAssignment {
		t.Fatalf("exam = %#v", exam)
	}
	if len(exam.Sections) != 1 || exam.Sections[0].Code != model.SectionCodeFull || len(exam.Sections[0].Questions) != 1 {
		t.Fatalf("sections = %#v", exam.Sections)
	}
	question := exam.Sections[0].Questions[0]
	if question.Content != "1 + 1 = ?" || len(question.Options) != 4 {
		t.Fatalf("question = %#v", question)
	}
	for i, option := range question.Options {
		if option.IsCorrect != (i == 1) || option.Order != i+1 {
			t.Fatalf("option %d = %#v", i, option)
		}
	}
	if *question.Options[1].Content != "2" || question.Options[1].Key != model.OptionB {
		t.Fatalf("correct option = %#v", question.Options[1])
	}
}

func TestUpdatePrivateExamReplacesQuestionsAndAudits(t *testing.T) {
	repo := &fakeExamRepository{exam: &model.Exam{Title: "Old", Type: model.ExamTypeAssignment, Sections: []model.Section{{Code: model.SectionCodeFull}}}}
	explanation := " because "
	_, err := (&service{repo: repo}).updatePrivateExam(context.Background(), 7, dto.UpdatePrivateExamRequest{
		Title:     " New ",
		ActorID:   "teacher-1",
		Reason:    " fix typo ",
		Questions: []dto.UpdatePrivateExamQuestionRequest{{Content: "Q", Explanation: &explanation, Options: []string{"a", "b", "c", "d"}, CorrectOptionIndex: 3, ExcludedFromScore: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if repo.exam.Title != "New" || len(repo.replaced) != 1 {
		t.Fatalf("exam = %#v replaced = %#v", repo.exam, repo.replaced)
	}
	question := repo.replaced[0]
	if question.Explanation != "because" || !question.ExcludedFromScore || !question.Options[3].IsCorrect {
		t.Fatalf("question = %#v", question)
	}
	if len(repo.audits) != 1 || repo.audits[0].ExamID != 7 || repo.audits[0].Reason != "fix typo" || string(repo.audits[0].ScoreChanges) != "[]" {
		t.Fatalf("audits = %#v", repo.audits)
	}
}

func TestUpdatePrivateExamRequiresTitleAndQuestions(t *testing.T) {
	repo := &fakeExamRepository{exam: &model.Exam{Title: "Old", Type: model.ExamTypeAssignment}}
	_, err := (&service{repo: repo}).updatePrivateExam(context.Background(), 7, dto.UpdatePrivateExamRequest{Title: "New"})
	var httpErr *httpx.HTTPError
	if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusBadRequest || len(repo.audits) != 0 {
		t.Fatalf("err = %v audits = %d", err, len(repo.audits))
	}
}
