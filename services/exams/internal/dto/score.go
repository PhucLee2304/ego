package dto

import (
	"errors"
	"fmt"

	"ego/platform/valuex"
	"ego/services/exams/internal/model"
)

var (
	ErrScoreQuestionNotFound     = errors.New("[ERROR] Score question not found")
	ErrScoreQuestionSectionEmpty = errors.New("[ERROR] Score question section is required")
)

func CalculateAttemptScore(examType model.ExamType, answers []model.AttemptAnswer, questions map[uint]*model.Question, totalQuestions int) (float64, error) {
	correctAnswers := 0
	listeningCorrect := 0
	readingCorrect := 0

	for _, answer := range answers {
		if answer.IsCorrect == nil || !*answer.IsCorrect {
			continue
		}

		correctAnswers++
		if examType != model.ExamTypeTOEIC {
			continue
		}

		question := questions[answer.QuestionID]
		if question == nil {
			return 0, fmt.Errorf("%w: questionID=%d", ErrScoreQuestionNotFound, answer.QuestionID)
		}

		sectionCode, err := sectionCodeFromQuestion(question)
		if err != nil {
			return 0, fmt.Errorf("%w: questionID=%d", err, answer.QuestionID)
		}

		switch sectionCode {
		case model.SectionCodeListening:
			listeningCorrect++
		case model.SectionCodeReading:
			readingCorrect++
		default:
			return 0, fmt.Errorf("%w: questionID=%d", ErrScoreQuestionSectionEmpty, answer.QuestionID)
		}
	}

	if examType == model.ExamTypeTOEIC {
		return float64(calculateTOEICScore(listeningCorrect, readingCorrect)), nil
	}

	return calculateLinearScore(correctAnswers, totalQuestions, 10), nil
}

func calculateLinearScore(correctAnswers, totalQuestions int, maxScore float64) float64 {
	if totalQuestions <= 0 || correctAnswers <= 0 {
		return 0
	}
	if correctAnswers > totalQuestions {
		correctAnswers = totalQuestions
	}

	return valuex.RoundFloat((float64(correctAnswers)/float64(totalQuestions))*maxScore, 2)
}

func calculateTOEICScore(listeningCorrect, readingCorrect int) int {
	return calculateTOEICSectionScore(listeningCorrect) + calculateTOEICSectionScore(readingCorrect)
}

func calculateTOEICSectionScore(correctAnswers int) int {
	correctAnswers = clampCorrectAnswers(correctAnswers)
	if correctAnswers <= 2 {
		return 5
	}
	return (correctAnswers - 1) * 5
}

func clampCorrectAnswers(correctAnswers int) int {
	if correctAnswers < 0 {
		return 0
	}
	if correctAnswers > 100 {
		return 100
	}
	return correctAnswers
}

func sectionCodeFromQuestion(question *model.Question) (model.SectionCode, error) {
	if question.Section.Code != "" {
		return question.Section.Code, nil
	}
	if question.Part == nil {
		return "", ErrScoreQuestionSectionEmpty
	}

	sectionCode, ok := model.SectionCodeByPart(*question.Part)
	if !ok {
		return "", ErrScoreQuestionSectionEmpty
	}
	return sectionCode, nil
}
