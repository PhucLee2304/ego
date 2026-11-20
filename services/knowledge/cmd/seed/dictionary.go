package main

import (
	"fmt"
	"strings"
	"time"

	"ego/platform/logger"
	"ego/services/knowledge/internal/model"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

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
	SourceURLs []string      `json:"sourceUrls"`
}

// seedDictionary thêm các từ chưa có trong DB, ghi theo lô batchSize từ. Lô lỗi thì ghi
// lại từng từ để một từ hỏng không làm mất cả lô.
func seedDictionary(db *gorm.DB, path string, batchSize int) error {
	var rawWords []rawWord
	if ok, err := readSeedFile(path, &rawWords); !ok {
		return err
	}
	if batchSize <= 0 {
		batchSize = 250
	}

	var existing []string
	if err := db.Model(&model.Word{}).Pluck("word", &existing).Error; err != nil {
		return fmt.Errorf("load existing words: %w", err)
	}
	seen := make(map[string]bool, len(existing))
	for _, w := range existing {
		seen[strings.ToLower(strings.TrimSpace(w))] = true
	}

	start := time.Now()
	seeded, skipped, failed := 0, 0, 0
	batch := make([]*model.Word, 0, batchSize)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		err := db.Transaction(func(tx *gorm.DB) error {
			for _, w := range batch {
				if err := tx.Create(w).Error; err != nil {
					return fmt.Errorf("create word %q: %w", w.Word, err)
				}
			}
			return nil
		})
		if err == nil {
			seeded += len(batch)
		} else {
			logger.Log.Warn().Err(err).Msg("[SEED_DICTIONARY] Batch failed, retrying word by word")
			for _, w := range batch {
				if err := db.Create(w).Error; err != nil {
					failed++
				} else {
					seeded++
				}
			}
		}
		batch = batch[:0]
	}

	for i := range rawWords {
		rw := &rawWords[i]
		word := strings.ToLower(strings.TrimSpace(rw.Word))
		if word == "" {
			continue
		}
		if seen[word] {
			skipped++
			continue
		}
		seen[word] = true
		w := &model.Word{
			Word:       word,
			Phonetic:   optionalString(rw.Phonetic),
			SourceUrls: datatypes.JSONSlice[string](nonEmpty(rw.SourceURLs)),
		}
		for _, rp := range rw.Phonetics {
			if strings.TrimSpace(rp.Text) == "" && strings.TrimSpace(rp.Audio) == "" {
				continue
			}
			w.Phonetics = append(w.Phonetics, model.Phonetic{
				Text:        optionalString(rp.Text),
				AudioUrl:    optionalString(rp.Audio),
				SourceUrl:   optionalString(rp.SourceURL),
				LicenseName: optionalString(rp.License.Name),
				LicenseUrl:  optionalString(rp.License.URL),
			})
		}
		for _, rm := range rw.Meanings {
			meaning := model.Meaning{
				PartOfSpeech: model.PartOfSpeech(strings.ToLower(strings.TrimSpace(rm.PartOfSpeech))),
				Synonyms:     datatypes.JSONSlice[string](nonEmpty(rm.Synonyms)),
				Antonyms:     datatypes.JSONSlice[string](nonEmpty(rm.Antonyms)),
			}
			for _, rd := range rm.Definitions {
				if strings.TrimSpace(rd.Definition) == "" {
					continue
				}
				meaning.Definitions = append(meaning.Definitions, model.Definition{
					DefinitionText: rd.Definition,
					Example:        optionalString(rd.Example),
					Synonyms:       datatypes.JSONSlice[string](nonEmpty(rd.Synonyms)),
					Antonyms:       datatypes.JSONSlice[string](nonEmpty(rd.Antonyms)),
				})
			}
			w.Meanings = append(w.Meanings, meaning)
		}
		batch = append(batch, w)
		if len(batch) >= batchSize {
			flush()
		}
	}
	flush()

	logger.Log.Info().Dur("took", time.Since(start).Round(time.Millisecond)).
		Int("total", len(rawWords)).Int("seeded", seeded).Int("skipped", skipped).Int("failed", failed).
		Msg("[SEED_DICTIONARY] Completed")
	return nil
}
