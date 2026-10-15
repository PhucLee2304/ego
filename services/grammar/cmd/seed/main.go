package main

import (
	"context"
	"encoding/json"
	"log"
	"os"

	grammarConfig "ego/services/grammar/config"
	"ego/services/grammar/database"
	"ego/services/grammar/internal/model"
	"ego/services/grammar/internal/repository"
)

// rawUnit maps directly to the JSON schema of Grammar.json.
type rawUnit struct {
	Unit      int           `json:"unit"`
	Title     string        `json:"title"`
	Theory    string        `json:"theory"`
	Examples  []string      `json:"examples"`
	Practices []rawPractice `json:"practice"`
}

type rawPractice struct {
	Question    string   `json:"question"`
	Options     []string `json:"options"`
	Correct     string   `json:"correct"`
	Explanation string   `json:"explanation"`
}

func main() {
	log.Println("[SEED] Starting Grammar seed process...")

	cfg, err := grammarConfig.LoadAppConfig()
	if err != nil {
		log.Fatalf("[SEED] Failed to load config: %v", err)
	}

	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("[SEED] Failed to connect to database: %v", err)
	}

	if err := database.Migrate(db); err != nil {
		log.Fatalf("[SEED] Failed to migrate database: %v", err)
	}

	repo := repository.NewRepository(db)
	ctx := context.Background()

	// Check if already seeded
	count, err := repo.CountUnits(ctx)
	if err != nil {
		log.Fatalf("[SEED] Failed to count units: %v", err)
	}
	if count > 0 {
		log.Printf("[SEED] Database already has %d units. Skipping seed.", count)
		return
	}

	// Read JSON
	seedPath := cfg.SeedFilePath
	log.Printf("[SEED] Loading grammar data from: %s", seedPath)

	data, err := os.ReadFile(seedPath)
	if err != nil {
		log.Fatalf("[SEED] Failed to read seed file: %v", err)
	}

	var rawUnits []rawUnit
	if err := json.Unmarshal(data, &rawUnits); err != nil {
		log.Fatalf("[SEED] Failed to parse seed file: %v", err)
	}

	log.Printf("[SEED] Found %d units to seed", len(rawUnits))

	// Build model structs
	units := make([]*model.GrammarUnit, 0, len(rawUnits))
	for _, raw := range rawUnits {
		examples := make([]model.GrammarExample, 0, len(raw.Examples))
		for _, ex := range raw.Examples {
			examples = append(examples, model.GrammarExample{Content: ex})
		}

		practices := make([]model.GrammarPractice, 0, len(raw.Practices))
		for i, p := range raw.Practices {
			optsJSON, _ := json.Marshal(p.Options)
			practices = append(practices, model.GrammarPractice{
				Question:    p.Question,
				Options:     string(optsJSON),
				Correct:     p.Correct,
				Explanation: p.Explanation,
				SortOrder:   i,
			})
		}

		units = append(units, &model.GrammarUnit{
			Unit:      raw.Unit,
			Title:     raw.Title,
			Theory:    raw.Theory,
			Examples:  examples,
			Practices: practices,
		})
	}

	if err := repo.BulkCreateUnits(ctx, units); err != nil {
		log.Fatalf("[SEED] Failed to seed grammar units: %v", err)
	}

	log.Printf("[SEED] Successfully seeded %d grammar units", len(units))
}
