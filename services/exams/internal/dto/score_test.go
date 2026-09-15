package dto

import (
	"ego/services/exams/internal/model"
	"errors"
	"testing"
)

func TestCalculateAttemptScoreTHPT(t *testing.T) {
	trueValue := true
	answers := make([]model.AttemptAnswer, 37)
	for i := range answers {
		answers[i] = model.AttemptAnswer{QuestionID: uint(i + 1), IsCorrect: &trueValue}
	}

	got, err := CalculateAttemptScore(model.ExamTypeTHPT, answers, nil, 50)
	if err != nil {
		t.Fatalf("CalculateAttemptScore() error = %v", err)
	}
	if got != 7.4 {
		t.Fatalf("CalculateAttemptScore() = %v, want %v", got, 7.4)
	}
}

func TestCalculateAttemptScoreTOEIC(t *testing.T) {
	trueValue := true
	answers := []model.AttemptAnswer{
		{QuestionID: 1, IsCorrect: &trueValue},
		{QuestionID: 2, IsCorrect: &trueValue},
		{QuestionID: 3, IsCorrect: &trueValue},
		{QuestionID: 4, IsCorrect: &trueValue},
		{QuestionID: 5, IsCorrect: &trueValue},
	}
	questions := map[uint]*model.Question{
		1: {Part: partPtr(model.Part1)},
		2: {Part: partPtr(model.Part2)},
		3: {Part: partPtr(model.Part5)},
		4: {Part: partPtr(model.Part6)},
		5: {Part: partPtr(model.Part7)},
	}

	got, err := CalculateAttemptScore(model.ExamTypeTOEIC, answers, questions, 200)
	if err != nil {
		t.Fatalf("CalculateAttemptScore() error = %v", err)
	}
	if got != 15 {
		t.Fatalf("CalculateAttemptScore() = %v, want %v", got, 15.0)
	}
}

func TestCalculateAttemptScoreTOEICMissingQuestionSection(t *testing.T) {
	trueValue := true
	answers := []model.AttemptAnswer{
		{QuestionID: 1, IsCorrect: &trueValue},
	}
	questions := map[uint]*model.Question{
		1: {},
	}

	_, err := CalculateAttemptScore(model.ExamTypeTOEIC, answers, questions, 200)
	if !errors.Is(err, ErrScoreQuestionSectionEmpty) {
		t.Fatalf("CalculateAttemptScore() error = %v, want %v", err, ErrScoreQuestionSectionEmpty)
	}
}

func TestCalculateTOEICSectionScores(t *testing.T) {
	tests := []struct {
		name      string
		listening int
		reading   int
		want      int
	}{
		{name: "minimum", listening: 0, reading: 0, want: 10},
		{name: "listening one reading two", listening: 1, reading: 2, want: 10},
		{name: "three per section", listening: 3, reading: 3, want: 20},
		{name: "middle scores", listening: 75, reading: 75, want: 740},
		{name: "perfect", listening: 100, reading: 100, want: 990},
		{name: "capped", listening: 120, reading: 120, want: 990},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := calculateTOEICScore(test.listening, test.reading); got != test.want {
				t.Fatalf("calculateTOEICScore() = %v, want %v", got, test.want)
			}
		})
	}
}

func partPtr(part model.PartCode) *model.PartCode {
	return &part
}
