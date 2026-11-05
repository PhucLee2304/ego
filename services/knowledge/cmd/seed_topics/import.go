package main

import (
	"fmt"

	"ego/services/knowledge/internal/model"

	"gorm.io/gorm"
)

func importData(db *gorm.DB, data *seedData, uploads *uploadResult) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := resetTopics(tx); err != nil {
			return err
		}

		topicIDs := make(map[uint]uint, len(data.topics))
		for _, row := range data.topics {
			topic := model.Topic{
				Name:    row.name,
				IsAudio: row.isAudio,
				Url:     row.url,
			}
			if err := tx.Create(&topic).Error; err != nil {
				return fmt.Errorf("create topic csv id %d: %w", row.csvID, err)
			}
			topicIDs[row.csvID] = topic.ID
		}

		sectionIDs := make(map[uint]uint, len(data.sections))
		for _, row := range data.sections {
			topicID, ok := topicIDs[row.topicCSVID]
			if !ok {
				return fmt.Errorf("missing topic db id for section csv id %d topic csv id %d", row.csvID, row.topicCSVID)
			}

			section := model.Section{
				Name:    row.name,
				TopicID: topicID,
			}
			if err := tx.Create(&section).Error; err != nil {
				return fmt.Errorf("create section csv id %d: %w", row.csvID, err)
			}
			sectionIDs[row.csvID] = section.ID
		}

		lessonIDs := make(map[uint]uint, len(data.lessons))
		for _, row := range data.lessons {
			sectionID, ok := sectionIDs[row.sectionCSVID]
			if !ok {
				return fmt.Errorf("missing section db id for lesson csv id %d section csv id %d", row.csvID, row.sectionCSVID)
			}

			url, ok := uploads.lessonURLs[row.csvID]
			if !ok {
				return fmt.Errorf("missing uploaded lesson url for lesson csv id %d", row.csvID)
			}

			lesson := model.Lesson{
				Title:       row.title,
				Description: row.description,
				Subtitle:    row.subtitle,
				Url:         url,
				SectionID:   sectionID,
			}
			if err := tx.Create(&lesson).Error; err != nil {
				return fmt.Errorf("create lesson csv id %d: %w", row.csvID, err)
			}
			lessonIDs[row.csvID] = lesson.ID
		}

		transcripts := make([]model.Transcript, 0, len(data.transcripts))
		for _, row := range data.transcripts {
			lessonID, ok := lessonIDs[row.lessonCSVID]
			if !ok {
				return fmt.Errorf("missing lesson db id for transcript csv id %d lesson csv id %d", row.csvID, row.lessonCSVID)
			}

			transcripts = append(transcripts, model.Transcript{
				Content:   row.content,
				Order:     row.order,
				TimeStart: row.timeStart,
				TimeEnd:   row.timeEnd,
				Url:       uploads.transcriptURLs[row.csvID],
				LessonID:  lessonID,
			})
		}
		if len(transcripts) > 0 {
			if err := tx.CreateInBatches(transcripts, 1000).Error; err != nil {
				return fmt.Errorf("create transcripts: %w", err)
			}
		}

		return nil
	})
}

func resetTopics(tx *gorm.DB) error {
	models := []any{
		&model.Transcript{},
		&model.Lesson{},
		&model.Section{},
		&model.Topic{},
	}

	for _, m := range models {
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(m).Error; err != nil {
			return fmt.Errorf("reset topics data: %w", err)
		}
	}

	return nil
}
