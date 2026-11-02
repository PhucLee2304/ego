package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	vocabConfig "ego/services/vocabgram/config"
	"ego/services/vocabgram/database"
	"ego/services/vocabgram/internal/model"
	"ego/services/vocabgram/internal/repository"

	"gorm.io/gorm"
)

// ---------- Raw Dictionary JSON structures ----------

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

// ---------- Raw Grammar JSON structures ----------

type rawGrammarUnit struct {
	Unit      int                  `json:"unit"`
	Title     string               `json:"title"`
	Theory    string               `json:"theory"`
	Examples  []string             `json:"examples"`
	Practices []rawGrammarPractice `json:"practice"`
}

type rawGrammarPractice struct {
	Question    string   `json:"question"`
	Options     []string `json:"options"`
	Correct     string   `json:"correct"`
	Explanation string   `json:"explanation"`
}

func main() {
	targetFlag := flag.String("target", "", "Target to seed: dictionary, grammar, all")
	flag.Parse()

	target := *targetFlag
	if target == "" {
		target = os.Getenv("SEED_TARGET")
	}
	if target == "" {
		target = "all"
	}

	log.Printf("[SEED] Starting VocabGram seeder (target=%s)...", target)

	appConfig, err := vocabConfig.LoadAppConfig()
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

	repo := repository.NewRepository(db)

	switch strings.ToLower(target) {
	case "dictionary", "dict", "vocab":
		seedDictionary(db, appConfig)
	case "grammar":
		seedGrammar(repo, appConfig)
	case "all":
		seedDictionary(db, appConfig)
		seedGrammar(repo, appConfig)
	default:
		log.Fatalf("[SEED] Unknown target %q. Available: dictionary, grammar, all", target)
	}

	log.Println("[SEED] Seeder finished successfully.")
}

func seedDictionary(db *gorm.DB, appConfig *vocabConfig.AppConfig) {
	seedFilePath := appConfig.SeedDictionaryFilePath
	candidates := []string{
		seedFilePath,
		"/app/services/vocabgram/cmd/seed/dictionary_data_offline.json",
		"/app/services/dictionary/cmd/seed/dictionary_data_offline.json",
		"C:\\Users\\Admin\\Desktop\\AppTiengAnh\\dictionary_data_offline.json",
		"./dictionary_data_offline.json",
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
		log.Printf("[SEED_DICTIONARY] Warning: seed file not found in any candidates (%v). Skipping dictionary seed.", candidates)
		return
	}

	log.Printf("[SEED_DICTIONARY] Reading seed file: %s", foundPath)
	data, err := os.ReadFile(foundPath)
	if err != nil {
		log.Printf("[SEED_DICTIONARY] Failed to read seed file: %v", err)
		return
	}

	var rawWords []rawWord
	if err := json.Unmarshal(data, &rawWords); err != nil {
		log.Printf("[SEED_DICTIONARY] Failed to parse seed file: %v", err)
		return
	}

	log.Printf("[SEED_DICTIONARY] Loaded %d words from seed file", len(rawWords))

	batchSize := appConfig.SeedBatchSize
	if batchSize <= 0 {
		batchSize = 500
	}

	start := time.Now()
	seeded, failed := 0, 0

	for i, rw := range rawWords {
		if i > 0 && i%batchSize == 0 {
			log.Printf("[SEED_DICTIONARY] Progress: %d/%d words processed (seeded: %d, failed: %d)",
				i, len(rawWords), seeded, failed)
		}

		if err := seedSingleWord(db, &rw); err != nil {
			log.Printf("[SEED_DICTIONARY] Failed to seed word %q: %v", rw.Word, err)
			failed++
			continue
		}
		seeded++
	}

	log.Printf("[SEED_DICTIONARY] Completed in %s | Total: %d | Seeded: %d | Failed: %d",
		time.Since(start).Round(time.Millisecond), len(rawWords), seeded, failed)
}

func seedSingleWord(db *gorm.DB, rw *rawWord) error {
	var count int64
	if err := db.Model(&model.Word{}).Where("LOWER(word) = LOWER(?)", rw.Word).Count(&count).Error; err != nil {
		return fmt.Errorf("check existence: %w", err)
	}
	if count > 0 {
		return nil // already seeded
	}

	sourceURLsJSON, _ := json.Marshal(rw.SourceURLs)

	word := &model.Word{
		Word:        rw.Word,
		Phonetic:    rw.Phonetic,
		LicenseName: rw.License.Name,
		LicenseURL:  rw.License.URL,
		SourceURLs:  string(sourceURLsJSON),
	}

	for _, rp := range rw.Phonetics {
		if rp.Text == "" && rp.Audio == "" {
			continue
		}
		word.Phonetics = append(word.Phonetics, model.Phonetic{
			Text:        rp.Text,
			AudioPath:   rp.Audio,
			SourceURL:   rp.SourceURL,
			LicenseName: rp.License.Name,
			LicenseURL:  rp.License.URL,
		})
	}

	for _, rm := range rw.Meanings {
		meaning := model.Meaning{
			PartOfSpeech: rm.PartOfSpeech,
		}

		for _, s := range rm.Synonyms {
			if s != "" {
				meaning.Synonyms = append(meaning.Synonyms, model.MeaningSynonym{Word: s})
			}
		}
		for _, a := range rm.Antonyms {
			if a != "" {
				meaning.Antonyms = append(meaning.Antonyms, model.MeaningAntonym{Word: a})
			}
		}

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

func seedGrammar(repo repository.Repository, appConfig *vocabConfig.AppConfig) {
	ctx := context.Background()

	count, err := repo.CountUnits(ctx)
	if err != nil {
		log.Printf("[SEED_GRAMMAR] Failed to count units: %v", err)
		return
	}
	if count > 0 {
		log.Printf("[SEED_GRAMMAR] Database already has %d units. Skipping grammar seed.", count)
		return
	}

	seedFilePath := appConfig.SeedGrammarFilePath
	candidates := []string{
		seedFilePath,
		"/app/services/vocabgram/cmd/seed/Grammar.json",
		"/app/services/grammar/cmd/seed/Grammar.json",
		"C:\\Users\\Admin\\NguPhapTiengAnh\\Grammar.json",
		"./Grammar.json",
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
		log.Printf("[SEED_GRAMMAR] Warning: Grammar.json not found in candidates (%v). Skipping grammar seed.", candidates)
		return
	}

	log.Printf("[SEED_GRAMMAR] Reading seed file: %s", foundPath)
	data, err := os.ReadFile(foundPath)
	if err != nil {
		log.Printf("[SEED_GRAMMAR] Failed to read seed file: %v", err)
		return
	}

	var rawUnits []rawGrammarUnit
	if err := json.Unmarshal(data, &rawUnits); err != nil {
		log.Printf("[SEED_GRAMMAR] Failed to parse seed file: %v", err)
		return
	}

	log.Printf("[SEED_GRAMMAR] Loaded %d units from seed file", len(rawUnits))

	units := make([]*model.GrammarUnit, 0, len(rawUnits))
	for _, ru := range rawUnits {
		unit := &model.GrammarUnit{
			Unit:   ru.Unit,
			Title:  ru.Title,
			Theory: ru.Theory,
		}

		for _, ex := range ru.Examples {
			if ex != "" {
				unit.Examples = append(unit.Examples, model.GrammarExample{Content: ex})
			}
		}

		for i, rp := range ru.Practices {
			optsJSON, _ := json.Marshal(rp.Options)
			unit.Practices = append(unit.Practices, model.GrammarPractice{
				Question:    rp.Question,
				Options:     string(optsJSON),
				Correct:     rp.Correct,
				Explanation: rp.Explanation,
				SortOrder:   i,
			})
		}

		units = append(units, unit)
	}

	if err := repo.BulkCreateUnits(ctx, units); err != nil {
		log.Printf("[SEED_GRAMMAR] Failed to insert units: %v", err)
		return
	}

	log.Printf("[SEED_GRAMMAR] Successfully seeded %d grammar units!", len(units))
}

