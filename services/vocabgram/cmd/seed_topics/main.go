package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	vocabConfig "ego/services/vocabgram/config"
	"ego/services/vocabgram/database"
	"ego/services/vocabgram/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type rawTopic struct {
	ID          string   `json:"id"`
	Category    string   `json:"category"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Icon        string   `json:"icon"`
	Level       string   `json:"level"`
	Words       []string `json:"words"`
}

func main() {
	log.Println("[SEED_TOPICS] Starting VocabGram topics seeder...")

	appConfig, err := vocabConfig.LoadAppConfig()
	if err != nil {
		log.Fatalf("[SEED_TOPICS] Failed to load config: %v", err)
	}

	db, err := database.Connect(appConfig)
	if err != nil {
		log.Fatalf("[SEED_TOPICS] Failed to connect to database: %v", err)
	}

	if err := database.Migrate(db); err != nil {
		log.Fatalf("[SEED_TOPICS] Failed to migrate database: %v", err)
	}

	seedFilePath := os.Getenv("TOPICS_SEED_FILE_PATH")
	if seedFilePath == "" {
		seedFilePath = appConfig.SeedTopicsFilePath
	}
	if len(os.Args) > 1 {
		seedFilePath = os.Args[1]
	}

	candidates := []string{
		seedFilePath,
		"/app/services/vocabgram/cmd/seed/topics.json",
		"/app/services/dictionary/cmd/seed/topics.json",
		"C:\\Users\\Admin\\Desktop\\AppTiengAnh\\topics.json",
		"./topics.json",
	}
	foundPath := ""
	for _, c := range candidates {
		if c != "" {
			if _, err := os.Stat(c); err == nil {
				foundPath = c
				break
			}
		}
	}

	if foundPath == "" {
		log.Fatalf("[SEED_TOPICS] topics.json not found in any candidate path (%v)", candidates)
	}

	log.Printf("[SEED_TOPICS] Reading seed file: %s", foundPath)
	data, err := os.ReadFile(foundPath)
	if err != nil {
		log.Fatalf("[SEED_TOPICS] Failed to read seed file: %v", err)
	}

	var rawTopics []rawTopic
	if err := json.Unmarshal(data, &rawTopics); err != nil {
		log.Fatalf("[SEED_TOPICS] Failed to parse topics: %v", err)
	}

	log.Printf("[SEED_TOPICS] Found %d topics to process", len(rawTopics))

	start := time.Now()
	for i, rt := range rawTopics {
		if err := upsertTopic(db, rt, i); err != nil {
			log.Printf("[SEED_TOPICS] Failed on topic %s (%s): %v", rt.ID, rt.Name, err)
			continue
		}
		log.Printf("[SEED_TOPICS] Processed topic [%d/%d]: %s (%s) - %d words",
			i+1, len(rawTopics), rt.ID, rt.Name, len(rt.Words))
	}

	log.Printf("[SEED_TOPICS] Done in %s!", time.Since(start).Round(time.Millisecond))
}

func upsertTopic(db *gorm.DB, rt rawTopic, orderIdx int) error {
	return db.Transaction(func(tx *gorm.DB) error {
		topic := model.Topic{
			ID:          rt.ID,
			Category:    rt.Category,
			Name:        rt.Name,
			Description: rt.Description,
			Icon:        rt.Icon,
			Level:       rt.Level,
			OrderIndex:  orderIdx,
			WordCount:   len(rt.Words),
			UpdatedAt:   time.Now(),
		}

		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"category", "name", "description", "icon", "level", "order_index", "word_count", "updated_at",
			}),
		}).Create(&topic).Error; err != nil {
			return fmt.Errorf("upsert topic %s: %w", rt.ID, err)
		}

		if err := tx.Where("topic_id = ?", rt.ID).Delete(&model.TopicWord{}).Error; err != nil {
			return fmt.Errorf("delete old topic_words for %s: %w", rt.ID, err)
		}

		if len(rt.Words) == 0 {
			return nil
		}

		topicWords := make([]model.TopicWord, len(rt.Words))
		now := time.Now()
		for j, w := range rt.Words {
			topicWords[j] = model.TopicWord{
				TopicID:   rt.ID,
				Word:      strings.TrimSpace(w),
				OrderNum:  j,
				CreatedAt: now,
			}
		}

		if err := tx.CreateInBatches(topicWords, 200).Error; err != nil {
			return fmt.Errorf("batch insert topic_words for %s: %w", rt.ID, err)
		}

		return nil
	})
}

