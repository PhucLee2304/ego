// Command seed nạp từ điển offline, ngữ pháp và chủ đề từ vựng từ các file JSON trong
// thư mục -dir. Thiếu file nào thì bỏ qua phần đó. Chạy lại không tạo trùng: từ và unit
// đã có được giữ nguyên, chủ đề được cập nhật theo file.
package main

import (
	"flag"
	"path/filepath"
	"strings"

	"ego/platform/logger"
	"ego/services/knowledge/config"
	"ego/services/knowledge/database"
)

const (
	dictionaryFile = "dictionary_data_offline.json"
	grammarFile    = "Grammar.json"
	themesFile     = "topics.json"
)

func main() {
	logger.Setup(logger.LoggerConfig{Level: "info", Pretty: true})

	target := flag.String("target", "all", "dictionary, grammar, themes hoặc all")
	dir := flag.String("dir", "/app/services/knowledge/seed/offline", "Thư mục chứa các file JSON")
	batchSize := flag.Int("batch-size", 250, "Số từ ghi trong một transaction")
	flag.Parse()

	db, err := database.Connect(config.LoadAppConfig())
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[SEED] Failed to connect to database")
	}
	if err := database.Migrate(db); err != nil {
		logger.Log.Fatal().Err(err).Msg("[SEED] Failed to migrate database")
	}

	seeds := map[string]func() error{
		"grammar":    func() error { return seedGrammar(db, filepath.Join(*dir, grammarFile)) },
		"themes":     func() error { return seedThemes(db, filepath.Join(*dir, themesFile)) },
		"dictionary": func() error { return seedDictionary(db, filepath.Join(*dir, dictionaryFile), *batchSize) },
	}

	targets := []string{"grammar", "themes", "dictionary"}
	if t := strings.ToLower(strings.TrimSpace(*target)); t != "all" {
		if _, ok := seeds[t]; !ok {
			logger.Log.Fatal().Str("target", t).Msg("[SEED] Unknown target, use dictionary, grammar, themes or all")
		}
		targets = []string{t}
	}

	for _, t := range targets {
		if err := seeds[t](); err != nil {
			logger.Log.Fatal().Err(err).Str("target", t).Msg("[SEED] Seed failed")
		}
	}
	logger.Log.Info().Msg("[SEED] Offline seed completed")
}
