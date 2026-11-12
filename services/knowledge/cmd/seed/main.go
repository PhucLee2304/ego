package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ego/platform/logger"
	"ego/services/knowledge/config"
	"ego/services/knowledge/database"
	"ego/services/knowledge/internal/model"

	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
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

// ---------- Raw Topics/Themes JSON structures ----------

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
	logger.Setup(logger.LoggerConfig{Level: "info", Pretty: true})

	targetFlag := flag.String("target", "all", "Seed target: dictionary, grammar, themes, all")
	dictPathFlag := flag.String("dict-path", "", "Path to dictionary_data_offline.json")
	grammarPathFlag := flag.String("grammar-path", "", "Path to Grammar.json")
	themesPathFlag := flag.String("themes-path", "", "Path to topics.json / themes.json")
	dbHostFlag := flag.String("db-host", "", "Override DB Host (e.g. localhost when run from host)")
	dbPortFlag := flag.String("db-port", "", "Override DB Port (e.g. 5434 when run from host)")
	dbUserFlag := flag.String("db-user", "", "Override DB User")
	dbPassFlag := flag.String("db-password", "", "Override DB Password")
	dbNameFlag := flag.String("db-name", "", "Override DB Name")
	batchSizeFlag := flag.Int("batch-size", 250, "Batch size for inserting words")
	flag.Parse()

	target := strings.ToLower(strings.TrimSpace(*targetFlag))

	// Connect to database
	db, err := initDB(*dbHostFlag, *dbPortFlag, *dbUserFlag, *dbPassFlag, *dbNameFlag)
	if err != nil {
		log.Fatalf("[SEED] Failed to connect to database: %v", err)
	}

	// Ensure tables and unique indices are migrated
	if err := database.Migrate(db); err != nil {
		log.Fatalf("[SEED] Failed to run database migrations: %v", err)
	}

	switch target {
	case "dictionary", "dict", "words":
		seedDictionary(db, *dictPathFlag, *batchSizeFlag)
	case "grammar", "units":
		seedGrammar(db, *grammarPathFlag)
	case "themes", "theme", "topics", "topic":
		seedThemes(db, *themesPathFlag)
	case "all":
		seedGrammar(db, *grammarPathFlag)
		seedThemes(db, *themesPathFlag)
		seedDictionary(db, *dictPathFlag, *batchSizeFlag)
	default:
		log.Fatalf("[SEED] Unknown target %q. Available: dictionary, grammar, themes, all", target)
	}

	log.Println("[SEED] All requested seeding tasks completed successfully.")
}

func initDB(host, port, user, pass, dbName string) (*gorm.DB, error) {
	// Fallback to environment variables if flags not provided
	if host == "" {
		host = os.Getenv("DB_HOST")
	}
	if port == "" {
		port = os.Getenv("DB_PORT")
	}
	if user == "" {
		user = os.Getenv("DB_USER")
	}
	if pass == "" {
		pass = os.Getenv("DB_PASSWORD")
	}
	if dbName == "" {
		dbName = os.Getenv("DB_NAME")
	}

	// If still empty, load from config
	if host == "" || port == "" || user == "" || pass == "" || dbName == "" {
		cfg := config.LoadAppConfig()
		if host == "" {
			host = cfg.DBHost
		}
		if port == "" {
			port = cfg.DBPort
		}
		if user == "" {
			user = cfg.DBUser
		}
		if pass == "" {
			pass = cfg.DBPassword
		}
		if dbName == "" {
			dbName = cfg.DBName
		}
	}

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, pass, dbName)

	log.Printf("[SEED] Connecting to PostgreSQL at %s:%s (db: %s, user: %s)...", host, port, dbName, user)

	// Use quiet logger for GORM during bulk seed to prevent terminal spam
	gormCfg := &gorm.Config{
		TranslateError: true,
		Logger:         gormLogger.Default.LogMode(gormLogger.Warn),
	}

	return gorm.Open(postgres.Open(dsn), gormCfg)
}

func findExistingFile(userPath string, candidates []string) (string, error) {
	if userPath != "" {
		if _, err := os.Stat(userPath); err == nil {
			return userPath, nil
		}
		return "", fmt.Errorf("specified file does not exist: %s", userPath)
	}

	for _, c := range candidates {
		if c != "" {
			if _, err := os.Stat(c); err == nil {
				abs, _ := filepath.Abs(c)
				return abs, nil
			}
		}
	}

	return "", fmt.Errorf("file not found in candidate paths: %v", candidates)
}

func seedGrammar(db *gorm.DB, filePath string) {
	candidates := []string{
		filePath,
		os.Getenv("SEED_GRAMMAR_PATH"),
		`C:\Users\Admin\NguPhapTiengAnh\Grammar.json`,
		`../NguPhapTiengAnh/Grammar.json`,
		`./Grammar.json`,
		`/app/services/knowledge/seed/Grammar.json`,
		`/app/seed/Grammar.json`,
	}

	foundPath, err := findExistingFile(filePath, candidates)
	if err != nil {
		log.Printf("[SEED_GRAMMAR] Warning: %v. Skipping grammar seed.", err)
		return
	}

	log.Printf("[SEED_GRAMMAR] Reading grammar seed file: %s", foundPath)
	data, err := os.ReadFile(foundPath)
	if err != nil {
		log.Printf("[SEED_GRAMMAR] Error reading file: %v", err)
		return
	}

	var rawUnits []rawGrammarUnit
	if err := json.Unmarshal(data, &rawUnits); err != nil {
		log.Printf("[SEED_GRAMMAR] Error parsing JSON: %v", err)
		return
	}

	log.Printf("[SEED_GRAMMAR] Found %d grammar units in seed file", len(rawUnits))

	// Load existing unit numbers to prevent duplication
	var existingUnitNums []int
	if err := db.Model(&model.GrammarUnit{}).Pluck("unit", &existingUnitNums).Error; err != nil {
		log.Printf("[SEED_GRAMMAR] Failed to load existing units: %v", err)
		return
	}

	existingMap := make(map[int]struct{}, len(existingUnitNums))
	for _, u := range existingUnitNums {
		existingMap[u] = struct{}{}
	}

	seeded, skipped := 0, 0
	start := time.Now()

	for _, ru := range rawUnits {
		if _, exists := existingMap[ru.Unit]; exists {
			skipped++
			continue
		}

		unit := &model.GrammarUnit{
			Unit:   ru.Unit,
			Title:  ru.Title,
			Theory: ru.Theory,
		}

		for _, ex := range ru.Examples {
			if strings.TrimSpace(ex) != "" {
				unit.Examples = append(unit.Examples, model.GrammarExample{
					Content: ex,
				})
			}
		}

		for idx, p := range ru.Practices {
			options := datatypes.JSONSlice[string](p.Options)
			if options == nil {
				options = datatypes.JSONSlice[string]{}
			}

			unit.Practices = append(unit.Practices, model.GrammarPractice{
				Question:    p.Question,
				Options:     options,
				Correct:     p.Correct,
				Explanation: p.Explanation,
				SortOrder:   idx,
			})
		}

		if err := db.Create(unit).Error; err != nil {
			log.Printf("[SEED_GRAMMAR] Failed to seed Unit %d (%s): %v", ru.Unit, ru.Title, err)
			continue
		}

		existingMap[ru.Unit] = struct{}{}
		seeded++
	}

	log.Printf("[SEED_GRAMMAR] Completed in %s | Total: %d | Seeded: %d | Skipped (already exist): %d",
		time.Since(start).Round(time.Millisecond), len(rawUnits), seeded, skipped)
}

func seedDictionary(db *gorm.DB, filePath string, batchSize int) {
	candidates := []string{
		filePath,
		os.Getenv("SEED_DICTIONARY_PATH"),
		`C:\Users\Admin\Desktop\AppTiengAnh\dictionary_data_offline.json`,
		`../Desktop/AppTiengAnh/dictionary_data_offline.json`,
		`./dictionary_data_offline.json`,
		`/app/services/knowledge/seed/dictionary_data_offline.json`,
		`/app/seed/dictionary_data_offline.json`,
	}

	foundPath, err := findExistingFile(filePath, candidates)
	if err != nil {
		log.Printf("[SEED_DICTIONARY] Warning: %v. Skipping dictionary seed.", err)
		return
	}

	log.Printf("[SEED_DICTIONARY] Reading dictionary file (this may take a few seconds): %s", foundPath)
	data, err := os.ReadFile(foundPath)
	if err != nil {
		log.Printf("[SEED_DICTIONARY] Error reading file: %v", err)
		return
	}

	var rawWords []rawWord
	if err := json.Unmarshal(data, &rawWords); err != nil {
		log.Printf("[SEED_DICTIONARY] Error parsing JSON: %v", err)
		return
	}

	log.Printf("[SEED_DICTIONARY] Loaded %d words from JSON", len(rawWords))

	// Fast pre-flight: fetch all existing word strings in DB
	log.Printf("[SEED_DICTIONARY] Fetching existing words from database...")
	var existingWords []string
	if err := db.Model(&model.Word{}).Pluck("word", &existingWords).Error; err != nil {
		log.Printf("[SEED_DICTIONARY] Failed to fetch existing words: %v", err)
		return
	}

	existingMap := make(map[string]struct{}, len(existingWords))
	for _, w := range existingWords {
		existingMap[strings.ToLower(strings.TrimSpace(w))] = struct{}{}
	}
	log.Printf("[SEED_DICTIONARY] Found %d words already in database", len(existingMap))

	if batchSize <= 0 {
		batchSize = 250
	}

	start := time.Now()
	seeded := 0
	skipped := 0
	failed := 0

	var batch []*model.Word

	flushBatch := func() {
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

		if err != nil {
			log.Printf("[SEED_DICTIONARY] Batch failed: %v. Retrying word-by-word...", err)
			for _, w := range batch {
				if err := db.Create(w).Error; err != nil {
					failed++
				} else {
					seeded++
				}
			}
		} else {
			seeded += len(batch)
		}

		batch = batch[:0]
	}

	for i, rw := range rawWords {
		cleanWord := strings.ToLower(strings.TrimSpace(rw.Word))
		if cleanWord == "" {
			continue
		}

		if _, exists := existingMap[cleanWord]; exists {
			skipped++
			continue
		}

		word := buildWordModel(&rw, cleanWord)
		batch = append(batch, word)
		existingMap[cleanWord] = struct{}{}

		if len(batch) >= batchSize {
			flushBatch()
		}

		if (i+1)%1000 == 0 || i+1 == len(rawWords) {
			elapsed := time.Since(start).Seconds()
			rate := float64(seeded+skipped) / elapsed
			log.Printf("[SEED_DICTIONARY] Progress: %d/%d processed | Seeded: %d | Skipped: %d | Failed: %d (%.0f words/s)",
				i+1, len(rawWords), seeded, skipped, failed, rate)
		}
	}

	flushBatch()

	log.Printf("[SEED_DICTIONARY] Completed in %s | Total: %d | Seeded: %d | Skipped: %d | Failed: %d",
		time.Since(start).Round(time.Millisecond), len(rawWords), seeded, skipped, failed)
}

func buildWordModel(rw *rawWord, cleanWord string) *model.Word {
	var phoneticPtr *string
	if rw.Phonetic != "" {
		p := rw.Phonetic
		phoneticPtr = &p
	}

	sourceURLs := datatypes.JSONSlice[string](rw.SourceURLs)
	if sourceURLs == nil {
		sourceURLs = datatypes.JSONSlice[string]{}
	}

	word := &model.Word{
		Word:       cleanWord,
		Phonetic:   phoneticPtr,
		SourceUrls: sourceURLs,
	}

	for _, rp := range rw.Phonetics {
		if rp.Text == "" && rp.Audio == "" {
			continue
		}

		var textPtr *string
		if rp.Text != "" {
			t := rp.Text
			textPtr = &t
		}

		var audioPtr *string
		if rp.Audio != "" {
			a := rp.Audio
			audioPtr = &a
		}

		var sourcePtr *string
		if rp.SourceURL != "" {
			s := rp.SourceURL
			sourcePtr = &s
		}

		var licNamePtr *string
		if rp.License.Name != "" {
			ln := rp.License.Name
			licNamePtr = &ln
		}

		var licURLPtr *string
		if rp.License.URL != "" {
			lu := rp.License.URL
			licURLPtr = &lu
		}

		word.Phonetics = append(word.Phonetics, model.Phonetic{
			Text:        textPtr,
			AudioUrl:    audioPtr,
			SourceUrl:   sourcePtr,
			LicenseName: licNamePtr,
			LicenseUrl:  licURLPtr,
		})
	}

	for _, rm := range rw.Meanings {
		pos := model.PartOfSpeech(strings.ToLower(strings.TrimSpace(rm.PartOfSpeech)))

		synonyms := filterNonEmpty(rm.Synonyms)
		antonyms := filterNonEmpty(rm.Antonyms)

		meaning := model.Meaning{
			PartOfSpeech: pos,
			Synonyms:     datatypes.JSONSlice[string](synonyms),
			Antonyms:     datatypes.JSONSlice[string](antonyms),
		}

		for _, rd := range rm.Definitions {
			if strings.TrimSpace(rd.Definition) == "" {
				continue
			}

			var exPtr *string
			if strings.TrimSpace(rd.Example) != "" {
				e := rd.Example
				exPtr = &e
			}

			defSynonyms := filterNonEmpty(rd.Synonyms)
			defAntonyms := filterNonEmpty(rd.Antonyms)

			meaning.Definitions = append(meaning.Definitions, model.Definition{
				DefinitionText: rd.Definition,
				Example:        exPtr,
				Synonyms:       datatypes.JSONSlice[string](defSynonyms),
				Antonyms:       datatypes.JSONSlice[string](defAntonyms),
			})
		}

		word.Meanings = append(word.Meanings, meaning)
	}

	return word
}

func filterNonEmpty(slice []string) []string {
	if len(slice) == 0 {
		return []string{}
	}
	res := make([]string, 0, len(slice))
	for _, s := range slice {
		trimmed := strings.TrimSpace(s)
		if trimmed != "" {
			res = append(res, trimmed)
		}
	}
	return res
}

func seedThemes(db *gorm.DB, filePath string) {
	candidates := []string{
		filePath,
		os.Getenv("SEED_THEMES_PATH"),
		os.Getenv("SEED_TOPICS_PATH"),
		`C:\Users\Admin\Desktop\AppTiengAnh\topics.json`,
		`../Desktop/AppTiengAnh/topics.json`,
		`./topics.json`,
		`/app/services/knowledge/seed/topics.json`,
		`/app/seed/topics.json`,
	}

	foundPath, err := findExistingFile(filePath, candidates)
	if err != nil {
		log.Printf("[SEED_THEMES] Warning: %v. Skipping themes seed.", err)
		return
	}

	log.Printf("[SEED_THEMES] Reading themes file: %s", foundPath)
	data, err := os.ReadFile(foundPath)
	if err != nil {
		log.Printf("[SEED_THEMES] Error reading file: %v", err)
		return
	}

	var rawTopics []rawTopic
	if err := json.Unmarshal(data, &rawTopics); err != nil {
		log.Printf("[SEED_THEMES] Error parsing JSON: %v", err)
		return
	}

	log.Printf("[SEED_THEMES] Loaded %d themes from JSON", len(rawTopics))
	start := time.Now()
	totalWords := 0
	seededThemes := 0

	err = db.Transaction(func(tx *gorm.DB) error {
		for i, rt := range rawTopics {
			cat := strings.TrimSpace(rt.Category)
			name := strings.TrimSpace(rt.Name)
			if name == "" {
				continue
			}

			var descPtr, iconPtr, levelPtr *string
			if strings.TrimSpace(rt.Description) != "" {
				d := strings.TrimSpace(rt.Description)
				descPtr = &d
			}
			if strings.TrimSpace(rt.Icon) != "" {
				ic := strings.TrimSpace(rt.Icon)
				iconPtr = &ic
			}
			if strings.TrimSpace(rt.Level) != "" {
				lvl := strings.TrimSpace(rt.Level)
				levelPtr = &lvl
			}

			// Deduplicate words and build theme_words list
			seen := make(map[string]bool)
			var rawCleanWords []string
			for _, rw := range rt.Words {
				w := strings.TrimSpace(strings.ToLower(rw))
				if w == "" || seen[w] {
					continue
				}
				seen[w] = true
				rawCleanWords = append(rawCleanWords, w)
			}

			var theme model.Theme
			err := tx.Where("name = ? AND category = ?", name, cat).First(&theme).Error
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					theme = model.Theme{
						Category:    cat,
						Name:        name,
						Description: descPtr,
						Icon:        iconPtr,
						Level:       levelPtr,
						OrderIndex:  i + 1,
						WordCount:   len(rawCleanWords),
					}
					if err := tx.Create(&theme).Error; err != nil {
						return fmt.Errorf("failed to create theme %s: %w", name, err)
					}
				} else {
					return fmt.Errorf("failed to query theme %s: %w", name, err)
				}
			} else {
				theme.Description = descPtr
				theme.Icon = iconPtr
				theme.Level = levelPtr
				theme.OrderIndex = i + 1
				theme.WordCount = len(rawCleanWords)
				if err := tx.Save(&theme).Error; err != nil {
					return fmt.Errorf("failed to update theme %s: %w", name, err)
				}
			}

			// Clean existing words for this theme
			if err := tx.Where("theme_id = ?", theme.ID).Delete(&model.ThemeWord{}).Error; err != nil {
				return fmt.Errorf("failed to clear existing theme_words for %s: %w", name, err)
			}

			// Insert theme words
			var themeWords []model.ThemeWord
			for idx, w := range rawCleanWords {
				themeWords = append(themeWords, model.ThemeWord{
					ThemeID:  theme.ID,
					Word:     w,
					OrderNum: idx + 1,
				})
			}

			if len(themeWords) > 0 {
				if err := tx.CreateInBatches(themeWords, 100).Error; err != nil {
					return fmt.Errorf("failed to insert theme words for %s: %w", name, err)
				}
			}

			totalWords += len(themeWords)
			seededThemes++
			log.Printf("[SEED_THEMES] Seeded theme '%s' (ID: %d) with %d words", theme.Name, theme.ID, len(themeWords))
		}
		return nil
	})

	if err != nil {
		log.Printf("[SEED_THEMES] Transaction failed: %v", err)
		return
	}

	log.Printf("[SEED_THEMES] Completed in %s | Seeded %d themes and %d words",
		time.Since(start).Round(time.Millisecond), seededThemes, totalWords)
}
