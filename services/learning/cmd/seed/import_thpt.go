package main

import (
	"fmt"
	"sort"
	"strings"

	"ego/services/learning/internal/model"

	"gorm.io/gorm"
)

func importData(db *gorm.DB, data *seedData, uploads *uploadResult, audio map[string]audioInfo) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := resetExams(tx); err != nil {
			return err
		}

		if err := importTHPT(tx, data); err != nil {
			return err
		}
		if err := importTOEIC(tx, data, uploads, audio); err != nil {
			return err
		}

		return nil
	})
}

func importTHPT(tx *gorm.DB, data *seedData) error {
	questionsByTest := make(map[uint][]thptQuestionRow)
	for _, row := range data.thptQuestions {
		questionsByTest[row.TestID] = append(questionsByTest[row.TestID], row)
	}
	for _, rows := range questionsByTest {
		sort.SliceStable(rows, func(i, j int) bool {
			if rows[i].QuestionNo == rows[j].QuestionNo {
				return rows[i].RowOrder < rows[j].RowOrder
			}
			return rows[i].QuestionNo < rows[j].QuestionNo
		})
	}

	answersByQuestion := make(map[uint][]thptAnswerRow)
	for _, row := range data.thptAnswers {
		answersByQuestion[row.QuestionID] = append(answersByQuestion[row.QuestionID], row)
	}
	for _, rows := range answersByQuestion {
		sort.SliceStable(rows, func(i, j int) bool {
			return optionOrderFromKey(rows[i].Value) < optionOrderFromKey(rows[j].Value)
		})
	}

	for _, test := range data.thptTests {
		exam := model.Exam{
			Title:       test.Title,
			Description: nil,
			IsPublic:    true,
			Type:        model.ExamTypeTHPT,
			Year:        extractYear(test.Title),
		}
		if err := tx.Create(&exam).Error; err != nil {
			return fmt.Errorf("create thpt exam %d: %w", test.ID, err)
		}

		duration := defaultTHPTDuration
		section := model.Section{
			Code:     model.SectionCodeFull,
			Title:    thptSectionTitle,
			Order:    1,
			Duration: &duration,
			ExamID:   exam.ID,
		}
		if err := tx.Create(&section).Error; err != nil {
			return fmt.Errorf("create thpt section for exam %d: %w", test.ID, err)
		}

		options := make([]model.Option, 0, 200)
		for _, row := range questionsByTest[test.ID] {
			question := model.Question{
				Content:     row.Content,
				Explanation: "",
				Order:       row.QuestionNo,
				SectionID:   section.ID,
			}
			if err := tx.Create(&question).Error; err != nil {
				return fmt.Errorf("create thpt question %d: %w", row.ID, err)
			}

			answers := answersByQuestion[row.ID]
			if len(answers) == 0 {
				return fmt.Errorf("thpt question %d has no answers", row.ID)
			}

			for _, answer := range answers {
				key, err := parseOptionKey(answer.Value)
				if err != nil {
					return fmt.Errorf("thpt question %d: %w", row.ID, err)
				}

				options = append(options, model.Option{
					Key:        key,
					Content:    stringPointer(normalizeOptionContent(answer.Value, answer.Text)),
					IsCorrect:  strings.EqualFold(answer.Value, row.CorrectAnswer),
					Order:      optionOrderFromKey(answer.Value),
					QuestionID: question.ID,
				})
			}
		}

		if len(options) > 0 {
			if err := tx.CreateInBatches(options, 1000).Error; err != nil {
				return fmt.Errorf("create thpt options for exam %d: %w", test.ID, err)
			}
		}
	}

	return nil
}

func resetExams(tx *gorm.DB) error {
	models := []any{
		&model.Option{},
		&model.Question{},
		&model.Group{},
		&model.Section{},
		&model.Exam{},
	}

	for _, m := range models {
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(m).Error; err != nil {
			return fmt.Errorf("reset exams data: %w", err)
		}
	}

	return nil
}
