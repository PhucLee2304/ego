package main

import (
	"fmt"
	"sort"
	"strings"

	"ego/services/learning/internal/model"

	"gorm.io/gorm"
)

func importTOEIC(tx *gorm.DB, data *seedData, uploads *uploadResult) error {
	questionsByTest := make(map[string][]toeicQuestionRow)
	for _, row := range data.toeicQuestions {
		questionsByTest[row.TestSetID] = append(questionsByTest[row.TestSetID], row)
	}
	for _, rows := range questionsByTest {
		sort.SliceStable(rows, func(i, j int) bool {
			return rows[i].RowOrder < rows[j].RowOrder
		})
	}

	answersByQuestion := make(map[string][]toeicAnswerRow)
	for _, row := range data.toeicAnswers {
		answersByQuestion[row.QuestionID] = append(answersByQuestion[row.QuestionID], row)
	}
	for _, rows := range answersByQuestion {
		sort.SliceStable(rows, func(i, j int) bool {
			return rows[i].AnswerOrder < rows[j].AnswerOrder
		})
	}

	for _, test := range data.toeicTests {
		exam := model.Exam{
			Title:       test.Title,
			Description: nil,
			IsPublic:    true,
			Type:        model.ExamTypeTOEIC,
			Year:        extractYear(test.Title),
		}
		if err := tx.Create(&exam).Error; err != nil {
			return fmt.Errorf("create toeic exam %s: %w", test.TestSetID, err)
		}

		listenDuration := defaultListenDuration
		readDuration := defaultReadDuration
		listeningSection := model.Section{
			Code:     model.SectionCodeListening,
			Title:    "Listening",
			Order:    1,
			Duration: &listenDuration,
			ExamID:   exam.ID,
		}
		readingSection := model.Section{
			Code:     model.SectionCodeReading,
			Title:    "Reading",
			Order:    2,
			Duration: &readDuration,
			ExamID:   exam.ID,
		}
		if err := tx.Create(&listeningSection).Error; err != nil {
			return fmt.Errorf("create listening section for exam %s: %w", test.TestSetID, err)
		}
		if err := tx.Create(&readingSection).Error; err != nil {
			return fmt.Errorf("create reading section for exam %s: %w", test.TestSetID, err)
		}

		sectionByCode := map[string]uint{
			string(model.SectionCodeListening): listeningSection.ID,
			string(model.SectionCodeReading):   readingSection.ID,
		}
		sectionOrderCounters := map[uint]int{
			listeningSection.ID: 0,
			readingSection.ID:   0,
		}
		groupIDs := make(map[string]uint)
		options := make([]model.Option, 0, 1000)

		for _, row := range questionsByTest[test.TestSetID] {
			sectionID, ok := sectionByCode[row.Section]
			if !ok {
				return fmt.Errorf("toeic question %s has unsupported section %q", row.QuestionID, row.Section)
			}

			var groupID *uint
			if strings.TrimSpace(row.GroupID) != "" {
				groupKey := test.TestSetID + ":" + row.GroupID
				if existingID, ok := groupIDs[groupKey]; ok {
					groupID = &existingID
				} else {
					audioURL := toPublicURLPointer(uploads, row.AudioURL)
					imageURL := toPublicURLPointer(uploads, row.ImageURL)
					group := model.Group{
						Title:       buildGroupTitle(row.Part, row.GroupOrder),
						Instruction: row.PassageText,
						AudioURL:    audioURL,
						ImageURL:    imageURL,
						Transcript:  row.AudioTranscript,
						Explanation: row.Explanation,
						Order:       row.GroupOrder,
						SectionID:   sectionID,
					}
					if err := tx.Create(&group).Error; err != nil {
						return fmt.Errorf("create toeic group %s: %w", groupKey, err)
					}
					groupIDs[groupKey] = group.ID
					groupID = &group.ID
				}
			}

			partCode, err := parsePartCode(row.Part)
			if err != nil {
				return fmt.Errorf("toeic question %s: %w", row.QuestionID, err)
			}

			sectionOrderCounters[sectionID]++
			question := model.Question{
				Content:     row.QuestionText,
				Part:        &partCode,
				Explanation: row.Explanation,
				Order:       sectionOrderCounters[sectionID],
				SectionID:   sectionID,
				GroupID:     groupID,
			}
			if err := tx.Create(&question).Error; err != nil {
				return fmt.Errorf("create toeic question %s: %w", row.QuestionID, err)
			}

			answers := answersByQuestion[row.QuestionID]
			if len(answers) == 0 {
				return fmt.Errorf("toeic question %s has no answers", row.QuestionID)
			}

			fallbackOptions := parseTranscriptOptions(row.AudioTranscript)
			for _, answer := range answers {
				key, err := optionKeyFromOrder(answer.AnswerOrder)
				if err != nil {
					return fmt.Errorf("toeic question %s: %w", row.QuestionID, err)
				}

				content := cleanText(answer.Content)
				if content == "" {
					content = fallbackOptions[key]
				}

				options = append(options, model.Option{
					Key:        key,
					Content:    nullableStringPointer(content),
					IsCorrect:  answer.IsCorrect || answer.AnswerOrder == row.CorrectAnswerOrder,
					Order:      answer.AnswerOrder,
					QuestionID: question.ID,
				})
			}
		}

		if len(options) > 0 {
			if err := tx.CreateInBatches(options, 1000).Error; err != nil {
				return fmt.Errorf("create toeic options for exam %s: %w", test.TestSetID, err)
			}
		}
	}

	return nil
}

func buildGroupTitle(part string, groupOrder int) string {
	part = strings.TrimSpace(strings.TrimPrefix(part, "PART_"))
	if part == "" {
		return fmt.Sprintf("Group %d", groupOrder)
	}
	return fmt.Sprintf("Part %s - Group %d", part, groupOrder)
}

func parseTranscriptOptions(transcript string) map[model.OptionKey]string {
	result := make(map[model.OptionKey]string)
	matches := transcriptOptionExp.FindAllStringSubmatchIndex(transcript, -1)
	for i, match := range matches {
		if len(match) < 4 {
			continue
		}

		key := transcript[match[2]:match[3]]
		contentStart := match[1]
		contentEnd := len(transcript)
		if i+1 < len(matches) {
			contentEnd = matches[i+1][0]
		}

		optionKey, err := parseOptionKey(key)
		if err != nil {
			continue
		}

		result[optionKey] = cleanText(transcript[contentStart:contentEnd])
	}
	return result
}

func parsePartCode(value string) (model.PartCode, error) {
	switch strings.TrimSpace(strings.ToUpper(value)) {
	case "PART_1":
		return model.Part1, nil
	case "PART_2":
		return model.Part2, nil
	case "PART_3":
		return model.Part3, nil
	case "PART_4":
		return model.Part4, nil
	case "PART_5":
		return model.Part5, nil
	case "PART_6":
		return model.Part6, nil
	case "PART_7":
		return model.Part7, nil
	default:
		return "", fmt.Errorf("unsupported part %q", value)
	}
}

func parseOptionKey(value string) (model.OptionKey, error) {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "A":
		return model.OptionA, nil
	case "B":
		return model.OptionB, nil
	case "C":
		return model.OptionC, nil
	case "D":
		return model.OptionD, nil
	default:
		return "", fmt.Errorf("unsupported option key %q", value)
	}
}

func optionKeyFromOrder(order int) (model.OptionKey, error) {
	switch order {
	case 1:
		return model.OptionA, nil
	case 2:
		return model.OptionB, nil
	case 3:
		return model.OptionC, nil
	case 4:
		return model.OptionD, nil
	default:
		return "", fmt.Errorf("unsupported option order %d", order)
	}
}

func optionOrderFromKey(value string) int {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "A":
		return 1
	case "B":
		return 2
	case "C":
		return 3
	case "D":
		return 4
	default:
		return 99
	}
}

func normalizeOptionContent(value string, text string) string {
	text = cleanText(text)
	value = strings.ToUpper(strings.TrimSpace(value))
	if text == "" {
		return value
	}

	prefixes := []string{
		value + ".",
		value + ")",
		value + ":",
		value + " ",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(strings.ToUpper(text), prefix) {
			return cleanText(strings.TrimSpace(text[len(prefix):]))
		}
	}

	return text
}
