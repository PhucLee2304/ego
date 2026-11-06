package repository

import (
	"context"
	"strings"

	"ego/platform/httpx"
	"ego/services/knowledge/internal/dto"
)

const statsInventorySQL = `
SELECT
	(SELECT COUNT(*) FROM topics WHERE deleted_at IS NULL) AS topics,
	(SELECT COUNT(*) FROM sections WHERE deleted_at IS NULL) AS sections,
	(SELECT COUNT(*) FROM lessons WHERE deleted_at IS NULL) AS lessons,
	(SELECT COUNT(*) FROM transcripts WHERE deleted_at IS NULL) AS transcripts,
	(SELECT COUNT(*) FROM words WHERE deleted_at IS NULL) AS words,
	(SELECT COUNT(*) FROM themes WHERE deleted_at IS NULL) AS themes,
	(SELECT COUNT(*) FROM grammar_units WHERE deleted_at IS NULL) AS grammar_units`

const statsFlashcardsSQL = `
SELECT
	COUNT(*) AS total,
	COUNT(*) FILTER (WHERE created_at >= @from AND created_at < @to) AS current,
	COUNT(*) FILTER (WHERE created_at >= @prev_from AND created_at < @prev_to) AS previous,
	COUNT(DISTINCT user_id) AS learners
FROM flashcards WHERE deleted_at IS NULL`

const statsFlashcardsDailySQL = `
SELECT to_char(created_at AT TIME ZONE @tz, 'YYYY-MM-DD') AS date, COUNT(*) AS count
FROM flashcards WHERE deleted_at IS NULL AND created_at >= @from AND created_at < @to
GROUP BY date ORDER BY date`

const statsGrammarSQL = `
SELECT
	COUNT(*) FILTER (WHERE updated_at >= @from AND updated_at < @to) AS current,
	COUNT(*) FILTER (WHERE updated_at >= @prev_from AND updated_at < @prev_to) AS previous,
	COUNT(DISTINCT user_id) FILTER (WHERE updated_at >= @from AND updated_at < @to) AS learners
FROM user_unit_progresses WHERE deleted_at IS NULL AND completed`

const statsTopUnitsSQL = `
SELECT u.id AS unit_id, u.unit, u.title, COUNT(*) AS completions
FROM user_unit_progresses p JOIN grammar_units u ON u.id = p.unit_id AND u.deleted_at IS NULL
WHERE p.deleted_at IS NULL AND p.completed AND p.updated_at >= @from AND p.updated_at < @to
GROUP BY u.id, u.unit, u.title ORDER BY completions DESC, u.unit LIMIT 5`

// GetAdminStats chạy các câu đếm cho màn thống kê admin.
func (r *repository) GetAdminStats(ctx context.Context, rng httpx.StatsRange) (*dto.AdminStatsResponse, error) {
	db := r.db.WithContext(ctx)
	args := map[string]any{
		"from": rng.From, "to": rng.To, "prev_from": rng.PrevFrom, "prev_to": rng.PrevTo, "tz": httpx.StatsTimeZone,
	}
	resp := &dto.AdminStatsResponse{}
	var flashcards struct {
		Total, Current, Previous, Learners int64
	}
	var grammar struct {
		Current, Previous, Learners int64
	}
	queries := []struct {
		sql string
		dst any
	}{
		{statsInventorySQL, &resp.Inventory},
		{statsFlashcardsSQL, &flashcards},
		{statsFlashcardsDailySQL, &resp.Flashcards.Daily},
		{statsGrammarSQL, &grammar},
		{statsTopUnitsSQL, &resp.Grammar.TopUnits},
	}
	for _, q := range queries {
		// Raw báo lỗi khi truyền map cho câu không có tham số @.
		values := []any{args}
		if !strings.Contains(q.sql, "@") {
			values = nil
		}
		if err := db.Raw(q.sql, values...).Scan(q.dst).Error; err != nil {
			return nil, err
		}
	}
	resp.Flashcards.Total = flashcards.Total
	resp.Flashcards.New = httpx.PeriodCount{Current: flashcards.Current, Previous: flashcards.Previous}
	resp.Flashcards.Learners = flashcards.Learners
	resp.Grammar.Completions = httpx.PeriodCount{Current: grammar.Current, Previous: grammar.Previous}
	resp.Grammar.Learners = grammar.Learners
	return resp, nil
}
