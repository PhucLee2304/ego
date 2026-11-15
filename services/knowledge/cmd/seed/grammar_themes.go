package main

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"ego/platform/logger"
	"ego/services/knowledge/internal/model"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

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

type rawTheme struct {
	Category    string   `json:"category"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Icon        string   `json:"icon"`
	Level       string   `json:"level"`
	Words       []string `json:"words"`
}

// seedGrammar thêm các unit chưa có (so theo số unit); unit đã có được giữ nguyên.
func seedGrammar(db *gorm.DB, path string) error {
	var rawUnits []rawGrammarUnit
	if ok, err := readSeedFile(path, &rawUnits); !ok {
		return err
	}

	var existing []int
	if err := db.Model(&model.GrammarUnit{}).Pluck("unit", &existing).Error; err != nil {
		return fmt.Errorf("load existing units: %w", err)
	}
	seen := make(map[int]bool, len(existing))
	for _, u := range existing {
		seen[u] = true
	}

	start := time.Now()
	seeded := 0
	for _, ru := range rawUnits {
		if seen[ru.Unit] {
			continue
		}
		unit := &model.GrammarUnit{Unit: ru.Unit, Title: ru.Title, Theory: ru.Theory}
		for _, ex := range nonEmpty(ru.Examples) {
			unit.Examples = append(unit.Examples, model.GrammarExample{Content: ex})
		}
		for i, p := range ru.Practices {
			unit.Practices = append(unit.Practices, model.GrammarPractice{
				Question:    p.Question,
				Options:     datatypes.JSONSlice[string](nonEmpty(p.Options)),
				Correct:     p.Correct,
				Explanation: p.Explanation,
				SortOrder:   i,
			})
		}
		if err := db.Create(unit).Error; err != nil {
			return fmt.Errorf("create unit %d: %w", ru.Unit, err)
		}
		seen[ru.Unit] = true
		seeded++
	}

	logger.Log.Info().Dur("took", time.Since(start).Round(time.Millisecond)).
		Int("total", len(rawUnits)).Int("seeded", seeded).Int("skipped", len(rawUnits)-seeded).
		Msg("[SEED_GRAMMAR] Completed")
	return nil
}

// seedThemes tạo hoặc cập nhật chủ đề (theo category + name) và thay toàn bộ danh sách từ
// của chủ đề theo file, trong một transaction.
func seedThemes(db *gorm.DB, path string) error {
	var rawThemes []rawTheme
	if ok, err := readSeedFile(path, &rawThemes); !ok {
		return err
	}

	start := time.Now()
	themes, words := 0, 0
	err := db.Transaction(func(tx *gorm.DB) error {
		for i, rt := range rawThemes {
			name := strings.TrimSpace(rt.Name)
			if name == "" {
				continue
			}
			category := strings.TrimSpace(rt.Category)
			themeWords := make([]string, 0, len(rt.Words))
			seen := make(map[string]bool, len(rt.Words))
			for _, w := range rt.Words {
				w = strings.ToLower(strings.TrimSpace(w))
				if w == "" || seen[w] {
					continue
				}
				seen[w] = true
				themeWords = append(themeWords, w)
			}

			var theme model.Theme
			err := tx.Where("name = ? AND category = ?", name, category).First(&theme).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("find theme %s: %w", name, err)
			}
			theme.Category = category
			theme.Name = name
			theme.Description = optionalString(rt.Description)
			theme.Icon = optionalString(rt.Icon)
			theme.Level = optionalString(rt.Level)
			theme.OrderIndex = i + 1
			theme.WordCount = len(themeWords)
			if err := tx.Save(&theme).Error; err != nil {
				return fmt.Errorf("save theme %s: %w", name, err)
			}

			// Xóa cứng: unique (theme_id, word) vẫn tính dòng đã xóa mềm.
			if err := tx.Unscoped().Where("theme_id = ?", theme.ID).Delete(&model.ThemeWord{}).Error; err != nil {
				return fmt.Errorf("clear words of theme %s: %w", name, err)
			}
			rows := make([]model.ThemeWord, len(themeWords))
			for j, w := range themeWords {
				rows[j] = model.ThemeWord{ThemeID: theme.ID, Word: w, OrderNum: j + 1}
			}
			if len(rows) > 0 {
				if err := tx.CreateInBatches(rows, 100).Error; err != nil {
					return fmt.Errorf("insert words of theme %s: %w", name, err)
				}
			}
			themes++
			words += len(rows)
		}
		return nil
	})
	if err != nil {
		return err
	}

	logger.Log.Info().Dur("took", time.Since(start).Round(time.Millisecond)).
		Int("themes", themes).Int("words", words).Msg("[SEED_THEMES] Completed")
	return nil
}
