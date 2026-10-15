package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	dictConfig "ego/services/dictionary/config"
	"ego/services/dictionary/database"
	"ego/services/dictionary/internal/model"

	"gorm.io/gorm"
)

// ---------- Raw JSON structures ----------

type rawLicense struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type rawPhonetic struct {
	Text      string     `json:"text"`
	Audio     string     `json:"audio"`
	SourceURL string     `json:"sourceUrl"`
	License   rawLicense `json:"license"`
}

type rawDefinition struct {
	Definition string   `json:"definition"`
	Example    string   `json:"example"`
	Synonyms   []string `json:"synonyms"`
	Antonyms   []string `json:"antonyms"`
}

type rawMeaning struct {
	PartOfSpeech string          `json:"partOfSpeech"`
	Definitions  []rawDefinition `json:"definitions"`
	Synonyms     []string        `json:"synonyms"`
	Antonyms     []string        `json:"antonyms"`
}

type rawWord struct {
	Word       string        `json:"word"`
	Phonetic   string        `json:"phonetic"`
	Phonetics  []rawPhonetic `json:"phonetics"`
	Meanings   []rawMeaning  `json:"meanings"`
	License    rawLicense    `json:"license"`
	SourceURLs []string      `json:"sourceUrls"`
}

func main() {
	log.Println("[SEED] Starting dictionary seeder...")

	appConfig, err := dictConfig.LoadAppConfig()
	if err != nil {
		log.Fatalf("[SEED] Failed to load config: %v", err)
	}

	db, err := database.Connect(appConfig)
	if err != nil {
		log.Fatalf("[SEED] Failed to connect to database: %v", err)
	}

	if err := database.Migrate(db); err != nil {
		log.Fatalf("[SEED] Failed to migrate database: %v", err)
	}

	seedFilePath := appConfig.SeedFilePath
	if len(os.Args) > 1 {
		seedFilePath = os.Args[1]
	}

	log.Printf("[SEED] Reading seed file: %s", seedFilePath)

	data, err := os.ReadFile(seedFilePath)
	if err != nil {
		log.Fatalf("[SEED] Failed to read seed file: %v", err)
	}

	var rawWords []rawWord
	if err := json.Unmarshal(data, &rawWords); err != nil {
		log.Fatalf("[SEED] Failed to parse seed file: %v", err)
	}

	log.Printf("[SEED] Loaded %d words from seed file", len(rawWords))

	batchSize := appConfig.SeedBatchSize
	if batchSize <= 0 {
		batchSize = 500
	}

	start := time.Now()
	seeded := 0
	skipped := 0
	failed := 0

	for i, rw := range rawWords {
		if i > 0 && i%batchSize == 0 {
			log.Printf("[SEED] Progress: %d/%d words processed (seeded: %d, skipped: %d, failed: %d)",
				i, len(rawWords), seeded, skipped, failed)
		}

		if err := seedWord(db, &rw); err != nil {
			log.Printf("[SEED] Failed to seed word %q: %v", rw.Word, err)
			failed++
			continue
		}
		seeded++
	}

	elapsed := time.Since(start)
	log.Printf("[SEED] Done! Total: %d | Seeded: %d | Skipped: %d | Failed: %d | Time: %s",
		len(rawWords), seeded, skipped, failed, elapsed.Round(time.Millisecond))
}

func seedWord(db *gorm.DB, rw *rawWord) error {
	// Skip if already exists
	var count int64
	if err := db.Model(&model.Word{}).Where("LOWER(word) = LOWER(?)", rw.Word).Count(&count).Error; err != nil {
		return fmt.Errorf("check existence: %w", err)
	}
	if count > 0 {
		return nil // already seeded
	}

	// Encode source URLs as JSON string
	sourceURLsJSON, _ := json.Marshal(rw.SourceURLs)

	word := &model.Word{
		Word:        rw.Word,
		Phonetic:    rw.Phonetic,
		LicenseName: rw.License.Name,
		LicenseURL:  rw.License.URL,
		SourceURLs:  string(sourceURLsJSON),
	}

	// Phonetics
	for _, rp := range rw.Phonetics {
		if rp.Text == "" && rp.Audio == "" {
			continue
		}
		word.Phonetics = append(word.Phonetics, model.Phonetic{
			Text:        rp.Text,
			AudioPath:   rp.Audio, // relative path e.g. "assets/audios/the-ca-unstressed.mp3"
			SourceURL:   rp.SourceURL,
			LicenseName: rp.License.Name,
			LicenseURL:  rp.License.URL,
		})
	}

	// Meanings
	for _, rm := range rw.Meanings {
		meaning := model.Meaning{
			PartOfSpeech: rm.PartOfSpeech,
		}

		// Meaning-level synonyms
		for _, s := range rm.Synonyms {
			if s != "" {
				meaning.Synonyms = append(meaning.Synonyms, model.MeaningSynonym{Word: s})
			}
		}
		// Meaning-level antonyms
		for _, a := range rm.Antonyms {
			if a != "" {
				meaning.Antonyms = append(meaning.Antonyms, model.MeaningAntonym{Word: a})
			}
		}

		// Definitions
		for _, rd := range rm.Definitions {
			if rd.Definition == "" {
				continue
			}
			def := model.Definition{
				Definition: rd.Definition,
				Example:    rd.Example,
			}
			for _, s := range rd.Synonyms {
				if s != "" {
					def.Synonyms = append(def.Synonyms, model.DefinitionSynonym{Word: s})
				}
			}
			for _, a := range rd.Antonyms {
				if a != "" {
					def.Antonyms = append(def.Antonyms, model.DefinitionAntonym{Word: a})
				}
			}
			meaning.Definitions = append(meaning.Definitions, def)
		}

		word.Meanings = append(word.Meanings, meaning)
	}

	return db.Create(word).Error
}
