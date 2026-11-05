package main

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type topicRow struct {
	csvID   uint
	name    string
	isAudio bool
	url     string
}

type sectionRow struct {
	csvID      uint
	topicCSVID uint
	name       string
}

type lessonRow struct {
	csvID          uint
	sectionCSVID   uint
	title          string
	description    string
	subtitle       string
	localMediaPath string
}

type transcriptRow struct {
	csvID          uint
	lessonCSVID    uint
	order          uint
	content        string
	timeStart      float64
	timeEnd        float64
	localAudioPath string
}

type seedData struct {
	topics      []topicRow
	sections    []sectionRow
	lessons     []lessonRow
	transcripts []transcriptRow
}

func loadSeedData(seedDir string) (*seedData, error) {
	topics, err := loadTopics(filepath.Join(seedDir, "db_topics.csv"))
	if err != nil {
		return nil, err
	}

	sections, skippedSections, err := loadSections(filepath.Join(seedDir, "db_sections.csv"))
	if err != nil {
		return nil, err
	}

	lessons, skippedLessons, err := loadLessons(filepath.Join(seedDir, "db_lessons.csv"), skippedSections)
	if err != nil {
		return nil, err
	}

	transcripts, err := loadTranscripts(filepath.Join(seedDir, "db_transcripts.csv"), skippedLessons)
	if err != nil {
		return nil, err
	}

	return &seedData{
		topics:      topics,
		sections:    sections,
		lessons:     lessons,
		transcripts: transcripts,
	}, nil
}

func loadTopics(path string) ([]topicRow, error) {
	records, err := readCSV(path)
	if err != nil {
		return nil, err
	}

	topics := make([]topicRow, 0, len(records))
	for _, record := range records {
		id, err := parseUint(record["id"], "topic id")
		if err != nil {
			return nil, err
		}
		if id == skippedTopicID {
			continue
		}

		isAudio, err := strconv.ParseBool(record["is_audio"])
		if err != nil {
			return nil, fmt.Errorf("invalid topic is_audio for id %d: %w", id, err)
		}

		topics = append(topics, topicRow{
			csvID:   id,
			name:    record["name"],
			isAudio: isAudio,
			url:     record["url"],
		})
	}

	return topics, nil
}

func loadSections(path string) ([]sectionRow, map[uint]struct{}, error) {
	records, err := readCSV(path)
	if err != nil {
		return nil, nil, err
	}

	sections := make([]sectionRow, 0, len(records))
	skipped := make(map[uint]struct{})
	for _, record := range records {
		id, err := parseUint(record["id"], "section id")
		if err != nil {
			return nil, nil, err
		}
		topicID, err := parseUint(record["topic_id"], "section topic_id")
		if err != nil {
			return nil, nil, err
		}
		if topicID == skippedTopicID {
			skipped[id] = struct{}{}
			continue
		}

		sections = append(sections, sectionRow{
			csvID:      id,
			topicCSVID: topicID,
			name:       record["name"],
		})
	}

	return sections, skipped, nil
}

func loadLessons(path string, skippedSections map[uint]struct{}) ([]lessonRow, map[uint]struct{}, error) {
	records, err := readCSV(path)
	if err != nil {
		return nil, nil, err
	}

	lessons := make([]lessonRow, 0, len(records))
	skipped := make(map[uint]struct{})
	for _, record := range records {
		id, err := parseUint(record["id"], "lesson id")
		if err != nil {
			return nil, nil, err
		}
		sectionID, err := parseUint(record["section_id"], "lesson section_id")
		if err != nil {
			return nil, nil, err
		}
		if _, ok := skippedSections[sectionID]; ok {
			skipped[id] = struct{}{}
			continue
		}

		lessons = append(lessons, lessonRow{
			csvID:          id,
			sectionCSVID:   sectionID,
			title:          record["title"],
			description:    record["description"],
			subtitle:       record["subtitle"],
			localMediaPath: strings.TrimSpace(record["local_full_media_path"]),
		})
	}

	return lessons, skipped, nil
}

func loadTranscripts(path string, skippedLessons map[uint]struct{}) ([]transcriptRow, error) {
	records, err := readCSV(path)
	if err != nil {
		return nil, err
	}

	transcripts := make([]transcriptRow, 0, len(records))
	for _, record := range records {
		id, err := parseUint(record["id"], "transcript id")
		if err != nil {
			return nil, err
		}
		lessonID, err := parseUint(record["lesson_id"], "transcript lesson_id")
		if err != nil {
			return nil, err
		}
		if _, ok := skippedLessons[lessonID]; ok {
			continue
		}

		order, err := parseUint(record["sequence_order"], "transcript sequence_order")
		if err != nil {
			return nil, err
		}
		timeStart, err := parseFloat(record["time_start"], "transcript time_start")
		if err != nil {
			return nil, err
		}
		timeEnd, err := parseFloat(record["time_end"], "transcript time_end")
		if err != nil {
			return nil, err
		}

		transcripts = append(transcripts, transcriptRow{
			csvID:          id,
			lessonCSVID:    lessonID,
			order:          order,
			content:        record["content"],
			timeStart:      timeStart,
			timeEnd:        timeEnd,
			localAudioPath: strings.TrimSpace(record["local_audio_path"]),
		})
	}

	return transcripts, nil
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

func parseUint(value, name string) (uint, error) {
	parsed, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", name, value, err)
	}
	return uint(parsed), nil
}

func parseFloat(value, name string) (float64, error) {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", name, value, err)
	}
	return parsed, nil
}
