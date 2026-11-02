package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	dictConfig "ego/services/dictionary/config"
	"ego/services/dictionary/database"
	"ego/services/dictionary/internal/model"

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
	log.Println("[SEED_TOPICS] Starting topics seeder...")

	appConfig, err := dictConfig.LoadAppConfig()
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
		if len(os.Args) > 1 {
			seedFilePath = os.Args[1]
		} else {
			candidates := []string{
				"/app/services/dictionary/cmd/seed/topics.json",
				"C:\\Users\\Admin\\Desktop\\AppTiengAnh\\topics.json",
				"./topics.json",
			}
			for _, c := range candidates {
				if _, err := os.Stat(c); err == nil {
					seedFilePath = c
					break
				}
			}
		}
	}
	if seedFilePath == "" {
		seedFilePath = "C:\\Users\\Admin\\Desktop\\AppTiengAnh\\topics.json"
	}

	log.Printf("[SEED_TOPICS] Reading topics file: %s", seedFilePath)

	data, err := os.ReadFile(seedFilePath)
	if err != nil {
		log.Fatalf("[SEED_TOPICS] Failed to read seed file: %v", err)
	}

	var rawTopics []rawTopic
	if err := json.Unmarshal(data, &rawTopics); err != nil {
		log.Fatalf("[SEED_TOPICS] Failed to parse topics file: %v", err)
	}

	log.Printf("[SEED_TOPICS] Loaded %d topics from file", len(rawTopics))

	start := time.Now()
	totalWords := 0

	err = db.Transaction(func(tx *gorm.DB) error {
		for i, rt := range rawTopics {
			// Clean existing words for this topic
			if err := tx.Where("topic_id = ?", rt.ID).Delete(&model.TopicWord{}).Error; err != nil {
				return fmt.Errorf("failed to clear existing topic_words for %s: %w", rt.ID, err)
			}

			// Deduplicate words and build topic_words list
			seen := make(map[string]bool)
			var topicWords []model.TopicWord
			for _, rawWord := range rt.Words {
				w := strings.TrimSpace(strings.ToLower(rawWord))
				if w == "" || seen[w] {
					continue
				}
				seen[w] = true
				topicWords = append(topicWords, model.TopicWord{
					TopicID:  rt.ID,
					Word:     w,
					OrderNum: len(topicWords) + 1,
				})
			}

			topic := model.Topic{
				ID:          rt.ID,
				Category:    rt.Category,
				Name:        rt.Name,
				Description: rt.Description,
				Icon:        rt.Icon,
				Level:       rt.Level,
				OrderIndex:  i + 1,
				WordCount:   len(topicWords),
			}

			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "id"}},
				DoUpdates: clause.AssignmentColumns([]string{"category", "name", "description", "icon", "level", "order_index", "word_count", "updated_at"}),
			}).Create(&topic).Error; err != nil {
				return fmt.Errorf("failed to upsert topic %s: %w", rt.ID, err)
			}

			if len(topicWords) > 0 {
				if err := tx.CreateInBatches(topicWords, 100).Error; err != nil {
					return fmt.Errorf("failed to insert topic words for %s: %w", rt.ID, err)
				}
			}

			totalWords += len(topicWords)
			log.Printf("[SEED_TOPICS] Seeded topic '%s' (%s) with %d words", rt.Name, rt.ID, len(topicWords))
		}
		return nil
	})

	if err != nil {
		log.Fatalf("[SEED_TOPICS] Transaction failed: %v", err)
	}

	log.Printf("[SEED_TOPICS] Completed! Seeded %d topics and %d words in %v", len(rawTopics), totalWords, time.Since(start))
}