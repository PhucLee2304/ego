package main

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func loadSeedData(seedDir string) (*seedData, error) {
	thptTests, err := loadTHPTTests(filepath.Join(seedDir, "db", "tests.csv"))
	if err != nil {
		return nil, err
	}

	thptQuestions, err := loadTHPTQuestions(filepath.Join(seedDir, "db", "questions.csv"))
	if err != nil {
		return nil, err
	}

	thptAnswers, err := loadTHPTAnswers(filepath.Join(seedDir, "db", "answers.csv"))
	if err != nil {
		return nil, err
	}

	toeicTests, err := loadTOEICTests(filepath.Join(seedDir, "db", "toeic_tests.csv"))
	if err != nil {
		return nil, err
	}

	toeicQuestions, err := loadTOEICQuestions(filepath.Join(seedDir, "db", "toeic_questions.csv"))
	if err != nil {
		return nil, err
	}

	toeicAnswers, err := loadTOEICAnswers(filepath.Join(seedDir, "db", "toeic_answers.csv"))
	if err != nil {
		return nil, err
	}

	if err := attachTOEICGroupFiles(filepath.Join(seedDir, "db", "toeic_group_files.csv"), toeicQuestions); err != nil {
		return nil, err
	}

	manifest, err := loadManifest(filepath.Join(seedDir, "media", "manifests", "toeic_media.json"))
	if err != nil {
		return nil, err
	}

	data := &seedData{
		thptTests:      thptTests,
		thptQuestions:  thptQuestions,
		thptAnswers:    thptAnswers,
		toeicTests:     toeicTests,
		toeicQuestions: toeicQuestions,
		toeicAnswers:   toeicAnswers,
		manifest:       manifest,
	}

	if err := verifyManifest(seedDir, data); err != nil {
		return nil, err
	}

	return data, nil
}

func loadTHPTTests(path string) ([]thptTestRow, error) {
	records, err := readCSV(path)
	if err != nil {
		return nil, err
	}

	rows := make([]thptTestRow, 0, len(records))
	for _, record := range records {
		id, err := parseUint(record["id"], "thpt test id")
		if err != nil {
			return nil, err
		}

		totalQuestions, err := parseInt(record["total_questions"], "thpt total_questions")
		if err != nil {
			return nil, err
		}

		rows = append(rows, thptTestRow{
			ID:             id,
			Title:          record["title"],
			TotalQuestions: totalQuestions,
		})
	}

	return rows, nil
}

func loadTHPTQuestions(path string) ([]thptQuestionRow, error) {
	records, err := readCSV(path)
	if err != nil {
		return nil, err
	}

	rows := make([]thptQuestionRow, 0, len(records))
	for i, record := range records {
		id, err := parseUint(record["id"], "thpt question id")
		if err != nil {
			return nil, err
		}
		testID, err := parseUint(record["test_id"], "thpt question test_id")
		if err != nil {
			return nil, err
		}
		questionNo, err := parseInt(record["question_number"], "thpt question_number")
		if err != nil {
			return nil, err
		}

		rows = append(rows, thptQuestionRow{
			ID:            id,
			TestID:        testID,
			QuestionNo:    questionNo,
			Content:       cleanText(record["content"]),
			CorrectAnswer: strings.ToUpper(strings.TrimSpace(record["correct_answer"])),
			RowOrder:      i + 1,
		})
	}

	return rows, nil
}

func loadTHPTAnswers(path string) ([]thptAnswerRow, error) {
	records, err := readCSV(path)
	if err != nil {
		return nil, err
	}

	rows := make([]thptAnswerRow, 0, len(records))
	for _, record := range records {
		id, err := parseUint(record["id"], "thpt answer id")
		if err != nil {
			return nil, err
		}
		questionID, err := parseUint(record["question_id"], "thpt answer question_id")
		if err != nil {
			return nil, err
		}

		rows = append(rows, thptAnswerRow{
			ID:         id,
			QuestionID: questionID,
			Value:      strings.ToUpper(strings.TrimSpace(record["answer_value"])),
			Text:       cleanText(record["answer_text"]),
		})
	}

	return rows, nil
}

func loadTOEICTests(path string) ([]toeicTestRow, error) {
	records, err := readCSV(path)
	if err != nil {
		return nil, err
	}

	rows := make([]toeicTestRow, 0, len(records))
	for _, record := range records {
		totalQuestions, err := parseInt(record["total_questions"], "toeic total_questions")
		if err != nil {
			return nil, err
		}

		rows = append(rows, toeicTestRow{
			TestSetID:      strings.TrimSpace(record["test_set_id"]),
			Slug:           strings.TrimSpace(record["test_slug"]),
			Title:          cleanText(record["title"]),
			TotalQuestions: totalQuestions,
		})
	}

	return rows, nil
}

func loadTOEICQuestions(path string) ([]toeicQuestionRow, error) {
	records, err := readCSV(path)
	if err != nil {
		return nil, err
	}

	rows := make([]toeicQuestionRow, 0, len(records))
	for i, record := range records {
		groupOrder, err := parseInt(record["group_order"], "toeic group_order")
		if err != nil {
			return nil, err
		}
		questionNo, err := parseInt(record["question_number"], "toeic question_number")
		if err != nil {
			return nil, err
		}
		correctOrder, err := parseInt(record["correct_answer_order"], "toeic correct_answer_order")
		if err != nil {
			return nil, err
		}

		rows = append(rows, toeicQuestionRow{
			QuestionID:         strings.TrimSpace(record["question_id"]),
			TestSetID:          strings.TrimSpace(record["test_set_id"]),
			GroupID:            strings.TrimSpace(record["group_id"]),
			Section:            strings.ToUpper(strings.TrimSpace(record["section"])),
			Part:               strings.ToUpper(strings.TrimSpace(record["part"])),
			GroupOrder:         groupOrder,
			QuestionNo:         questionNo,
			QuestionText:       cleanText(record["question_text"]),
			PassageText:        cleanText(record["passage_text"]),
			AudioTranscript:    cleanText(record["audio_transcript"]),
			Explanation:        cleanText(record["explanation"]),
			CorrectAnswerOrder: correctOrder,
			RowOrder:           i + 1,
		})
	}

	return rows, nil
}

// attachTOEICGroupFiles gán media của group (toeic_group_files.csv) cho từng câu:
// audio đầu tiên và mọi ảnh theo file_order.
func attachTOEICGroupFiles(path string, questions []toeicQuestionRow) error {
	records, err := readCSV(path)
	if err != nil {
		return err
	}

	type groupFile struct {
		order int
		url   string
	}
	audio := make(map[string]groupFile)
	images := make(map[string][]groupFile)
	for _, record := range records {
		order, err := parseInt(record["file_order"], "toeic file_order")
		if err != nil {
			return err
		}
		groupID := strings.TrimSpace(record["group_id"])
		file := groupFile{order: order, url: strings.TrimSpace(record["url"])}
		switch strings.ToUpper(strings.TrimSpace(record["file_type"])) {
		case "AUDIO":
			if current, ok := audio[groupID]; !ok || order < current.order {
				audio[groupID] = file
			}
		case "IMAGE":
			images[groupID] = append(images[groupID], file)
		default:
			return fmt.Errorf("unsupported toeic file_type %q for group %s", record["file_type"], groupID)
		}
	}

	for i := range questions {
		groupID := questions[i].GroupID
		questions[i].AudioURL = audio[groupID].url
		files := images[groupID]
		sort.Slice(files, func(a, b int) bool { return files[a].order < files[b].order })
		questions[i].ImageURLs = nil
		for _, file := range files {
			questions[i].ImageURLs = append(questions[i].ImageURLs, file.url)
		}
	}

	return nil
}

func loadTOEICAnswers(path string) ([]toeicAnswerRow, error) {
	records, err := readCSV(path)
	if err != nil {
		return nil, err
	}

	rows := make([]toeicAnswerRow, 0, len(records))
	for _, record := range records {
		answerOrder, err := parseInt(record["answer_order"], "toeic answer_order")
		if err != nil {
			return nil, err
		}
		isCorrect, err := strconv.ParseBool(strings.TrimSpace(record["is_correct"]))
		if err != nil {
			return nil, fmt.Errorf("invalid toeic is_correct %q: %w", record["is_correct"], err)
		}

		rows = append(rows, toeicAnswerRow{
			AnswerID:    strings.TrimSpace(record["answer_id"]),
			QuestionID:  strings.TrimSpace(record["question_id"]),
			AnswerOrder: answerOrder,
			Content:     cleanText(record["content"]),
			IsCorrect:   isCorrect,
		})
	}

	return rows, nil
}

func readCSV(path string) ([]map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1

	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read header %s: %w", path, err)
	}
	for i := range headers {
		headers[i] = strings.TrimSpace(headers[i])
	}

	var records []map[string]string
	for {
		row, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read row %s: %w", path, err)
		}

		record := make(map[string]string, len(headers))
		for i, header := range headers {
			if i < len(row) {
				record[header] = strings.TrimSpace(row[i])
			}
		}
		records = append(records, record)
	}

	return records, nil
}

func requireFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("missing media file %s: %w", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("media path is a directory: %s", path)
	}
	if info.Size() <= 0 {
		return fmt.Errorf("media file is empty: %s", path)
	}
	return nil
}

func cleanText(value string) string {
	value = strings.ReplaceAll(value, "\u00a0", " ")
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	return strings.TrimSpace(value)
}

func nullableStringPointer(value string) *string {
	value = cleanText(value)
	if value == "" {
		return nil
	}
	return &value
}

func stringPointer(value string) *string {
	return &value
}

func parseUint(value, name string) (uint, error) {
	parsed, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", name, value, err)
	}
	return uint(parsed), nil
}

func parseInt(value, name string) (int, error) {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", name, value, err)
	}
	return parsed, nil
}

func extractYear(value string) *int {
	match := yearPattern.FindString(value)
	if match == "" {
		return nil
	}
	year, err := strconv.Atoi(match)
	if err != nil {
		return nil
	}
	return &year
}
