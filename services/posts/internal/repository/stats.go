package repository

import (
	"context"
	"strings"

	"ego/platform/httpx"
	"ego/services/posts/internal/dto"
)

const statsPostsSQL = `
SELECT
	COUNT(*) FILTER (WHERE scope = 'COMMUNITY' AND status = 'PUBLISHED') AS total_posts,
	COUNT(*) FILTER (WHERE scope = 'COMMUNITY' AND created_at >= @from AND created_at < @to) AS current,
	COUNT(*) FILTER (WHERE scope = 'COMMUNITY' AND created_at >= @prev_from AND created_at < @prev_to) AS previous,
	COUNT(*) FILTER (WHERE scope = 'COMMUNITY' AND status = 'HIDDEN') AS hidden_posts,
	COUNT(*) FILTER (WHERE scope = 'CLASSROOM' AND created_at >= @from AND created_at < @to) AS classroom_posts
FROM posts WHERE deleted_at IS NULL`

const statsCommentsSQL = `
SELECT
	COUNT(*) FILTER (WHERE c.created_at >= @from AND c.created_at < @to) AS current,
	COUNT(*) FILTER (WHERE c.created_at >= @prev_from AND c.created_at < @prev_to) AS previous
FROM post_comments c JOIN posts p ON p.id = c.post_id AND p.deleted_at IS NULL AND p.scope = 'COMMUNITY'
WHERE c.deleted_at IS NULL`

const statsLikesSQL = `
SELECT
	COUNT(*) FILTER (WHERE l.created_at >= @from AND l.created_at < @to) AS current,
	COUNT(*) FILTER (WHERE l.created_at >= @prev_from AND l.created_at < @prev_to) AS previous
FROM post_likes l JOIN posts p ON p.id = l.post_id AND p.deleted_at IS NULL AND p.scope = 'COMMUNITY'`

const statsDailySQL = `
SELECT date, SUM(posts) AS posts, SUM(comments) AS comments FROM (
	SELECT to_char(created_at AT TIME ZONE @tz, 'YYYY-MM-DD') AS date, 1 AS posts, 0 AS comments
	FROM posts WHERE deleted_at IS NULL AND scope = 'COMMUNITY' AND created_at >= @from AND created_at < @to
	UNION ALL
	SELECT to_char(c.created_at AT TIME ZONE @tz, 'YYYY-MM-DD'), 0, 1
	FROM post_comments c JOIN posts p ON p.id = c.post_id AND p.deleted_at IS NULL AND p.scope = 'COMMUNITY'
	WHERE c.deleted_at IS NULL AND c.created_at >= @from AND c.created_at < @to
) t GROUP BY date ORDER BY date`

const statsAttachmentsSQL = `
SELECT a.media_type AS type, COUNT(*) AS count, COALESCE(SUM(a.size), 0) AS bytes
FROM post_attachments a JOIN posts p ON p.id = a.post_id AND p.deleted_at IS NULL AND p.scope = 'COMMUNITY'
WHERE a.deleted_at IS NULL AND p.created_at >= @from AND p.created_at < @to
GROUP BY a.media_type ORDER BY count DESC`

const statsTopAuthorsSQL = `
SELECT p.author_id AS user_id, COUNT(*) AS posts, COALESCE(SUM(l.count), 0) AS likes
FROM posts p
LEFT JOIN (SELECT post_id, COUNT(*) AS count FROM post_likes GROUP BY post_id) l ON l.post_id = p.id
WHERE p.deleted_at IS NULL AND p.scope = 'COMMUNITY' AND p.created_at >= @from AND p.created_at < @to
GROUP BY p.author_id ORDER BY posts DESC, likes DESC, p.author_id LIMIT 5`

// GetAdminStats chạy các câu đếm cho màn thống kê admin; User của top tác giả do service điền.
func (r *Repository) GetAdminStats(ctx context.Context, rng httpx.StatsRange) (*dto.AdminStatsResponse, error) {
	db := r.db.WithContext(ctx)
	args := map[string]any{
		"from": rng.From, "to": rng.To, "prev_from": rng.PrevFrom, "prev_to": rng.PrevTo, "tz": httpx.StatsTimeZone,
	}
	resp := &dto.AdminStatsResponse{}
	var posts struct {
		TotalPosts, Current, Previous, HiddenPosts, ClassroomPosts int64
	}
	queries := []struct {
		sql string
		dst any
	}{
		{statsPostsSQL, &posts},
		{statsCommentsSQL, &resp.Comments},
		{statsLikesSQL, &resp.Likes},
		{statsDailySQL, &resp.Daily},
		{statsAttachmentsSQL, &resp.Attachments},
		{statsTopAuthorsSQL, &resp.TopAuthors},
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
	resp.TotalPosts = posts.TotalPosts
	resp.NewPosts = httpx.PeriodCount{Current: posts.Current, Previous: posts.Previous}
	resp.HiddenPosts = posts.HiddenPosts
	resp.ClassroomPosts = posts.ClassroomPosts
	return resp, nil
}
