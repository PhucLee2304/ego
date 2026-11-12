package repository

import (
	"context"
	"strings"
	"time"

	"ego/platform/httpx"
	"ego/services/learning/internal/dto"
)

const statsClassroomsSQL = `
SELECT
	COUNT(*) FILTER (WHERE active AND created_at < @to) AS current,
	COUNT(*) FILTER (WHERE active AND created_at < @prev_to) AS previous,
	COUNT(*) FILTER (WHERE NOT active) AS archived,
	COUNT(DISTINCT teacher_id) FILTER (WHERE active) AS teachers
FROM classrooms WHERE deleted_at IS NULL`

const statsMembersSQL = `
SELECT
	COUNT(DISTINCT m.user_id) FILTER (WHERE m.approved AND COALESCE(m.approved_at, m.created_at) < @to) AS current,
	COUNT(DISTINCT m.user_id) FILTER (WHERE m.approved AND COALESCE(m.approved_at, m.created_at) < @prev_to) AS previous,
	COUNT(*) FILTER (WHERE m.approved IS NULL AND c.active) AS pending
FROM members m JOIN classrooms c ON c.id = m.classroom_id AND c.deleted_at IS NULL
WHERE m.deleted_at IS NULL`

const statsAssignmentsSQL = `
SELECT
	COUNT(*) AS total,
	COUNT(*) FILTER (WHERE a.active AND a.opens_at <= now() AND a.due_at > now()) AS open
FROM assignments a JOIN classrooms c ON c.id = a.classroom_id AND c.deleted_at IS NULL
WHERE a.deleted_at IS NULL`

// Bài nộp tính theo thời điểm nộp của attempt; đúng hạn là nộp trước due_at.
const statsSubmissionsSQL = `
SELECT
	COUNT(*) FILTER (WHERE t.submitted_at >= @from AND t.submitted_at < @to) AS current,
	COUNT(*) FILTER (WHERE t.submitted_at >= @prev_from AND t.submitted_at < @prev_to) AS previous,
	COUNT(*) FILTER (WHERE t.submitted_at >= @from AND t.submitted_at < @to AND t.submitted_at <= a.due_at) AS on_time,
	ROUND(AVG(s.score) FILTER (WHERE t.submitted_at >= @from AND t.submitted_at < @to), 2) AS current_score,
	ROUND(AVG(s.score) FILTER (WHERE t.submitted_at >= @prev_from AND t.submitted_at < @prev_to), 2) AS previous_score
FROM assignment_submissions s
JOIN attempts t ON t.id = s.attempt_id
JOIN assignments a ON a.id = s.assignment_id
WHERE s.deleted_at IS NULL AND s.score IS NOT NULL`

const statsScoreBucketsSQL = `
SELECT LEAST(GREATEST(FLOOR(s.score)::int, 0), 9) AS bucket, COUNT(*) AS count
FROM assignment_submissions s JOIN attempts t ON t.id = s.attempt_id
WHERE s.deleted_at IS NULL AND s.score IS NOT NULL AND t.submitted_at >= @from AND t.submitted_at < @to
GROUP BY bucket`

const statsNotSubmittedSQL = `
SELECT COALESCE(SUM(GREATEST(m.count - COALESCE(s.count, 0), 0)), 0)
FROM assignments a
JOIN (SELECT classroom_id, COUNT(*) AS count FROM members WHERE approved AND deleted_at IS NULL GROUP BY classroom_id) m
	ON m.classroom_id = a.classroom_id
LEFT JOIN (SELECT assignment_id, COUNT(*) AS count FROM assignment_submissions WHERE score IS NOT NULL AND deleted_at IS NULL GROUP BY assignment_id) s
	ON s.assignment_id = a.id
WHERE a.deleted_at IS NULL AND a.active AND a.opens_at <= now() AND a.due_at > now()`

const statsAttemptTotalsSQL = `
SELECT
	COUNT(*) FILTER (WHERE submitted_at >= @from AND submitted_at < @to) AS current,
	COUNT(*) FILTER (WHERE submitted_at >= @prev_from AND submitted_at < @prev_to) AS previous
FROM attempts WHERE deleted_at IS NULL AND status = 'SUBMITTED'`

const statsAttemptModesSQL = `
SELECT mode AS key, COUNT(*) AS count FROM attempts
WHERE deleted_at IS NULL AND status = 'SUBMITTED' AND submitted_at >= @from AND submitted_at < @to
GROUP BY mode ORDER BY count DESC`

const statsAttemptDailySQL = `
SELECT
	to_char(submitted_at AT TIME ZONE @tz, 'YYYY-MM-DD') AS date,
	COUNT(*) FILTER (WHERE mode = 'ASSIGNMENT') AS assignment,
	COUNT(*) FILTER (WHERE mode = 'PRACTICE') AS practice,
	COUNT(*) FILTER (WHERE mode = 'TEST') AS test
FROM attempts
WHERE deleted_at IS NULL AND status = 'SUBMITTED' AND submitted_at >= @from AND submitted_at < @to
GROUP BY date ORDER BY date`

// Top 10 đề nhiều lượt tự do nhất, xếp lại theo điểm TB để vẽ thanh ngang.
const statsExamAveragesSQL = `
SELECT * FROM (
	SELECT e.id AS exam_id, e.title, COUNT(*) AS attempts, ROUND(AVG(t.score), 2) AS average_score
	FROM attempts t JOIN exams e ON e.id = t.exam_id
	WHERE t.deleted_at IS NULL AND t.status = 'SUBMITTED' AND t.context_type = 'STANDALONE' AND t.score IS NOT NULL
		AND e.type = @type AND t.submitted_at >= @from AND t.submitted_at < @to
	GROUP BY e.id, e.title ORDER BY attempts DESC, e.id LIMIT 10
) top ORDER BY average_score DESC, exam_id`

const statsViolationsSQL = `
SELECT type AS key, COUNT(*) AS count FROM attempt_integrity_events
WHERE deleted_at IS NULL AND level = 'VIOLATION' AND received_at >= @from AND received_at < @to
GROUP BY type ORDER BY count DESC`

const statsTerminatedSQL = `
SELECT COUNT(*) FROM attempts
WHERE deleted_at IS NULL AND terminated_at >= @from AND terminated_at < @to`

const statsRoomsSQL = `
SELECT
	COUNT(*) AS sessions,
	COALESCE(ROUND((SUM(EXTRACT(EPOCH FROM ended_at - started_at)) / 3600)::numeric, 1), 0) AS hours,
	COALESCE(ROUND(AVG(p.count), 1), 0) AS avg_participants
FROM rooms r
LEFT JOIN (SELECT room_id, COUNT(DISTINCT user_id) AS count FROM room_participants WHERE deleted_at IS NULL GROUP BY room_id) p
	ON p.room_id = r.id
WHERE r.deleted_at IS NULL AND r.status = 'ENDED' AND r.started_at >= @from AND r.started_at < @to`

const statsClassroomListSQL = `
SELECT
	c.id, c.name, c.code, c.teacher_id, c.active, c.created_at,
	(SELECT COUNT(*) FROM members m WHERE m.classroom_id = c.id AND m.approved AND m.deleted_at IS NULL) AS students,
	(SELECT COUNT(*) FROM assignments a WHERE a.classroom_id = c.id AND a.deleted_at IS NULL) AS assignments,
	s.count AS submissions,
	s.average_score
FROM classrooms c
LEFT JOIN LATERAL (
	SELECT COUNT(*) AS count, ROUND(AVG(sub.score), 2) AS average_score
	FROM assignment_submissions sub JOIN assignments a ON a.id = sub.assignment_id AND a.deleted_at IS NULL
	WHERE a.classroom_id = c.id AND sub.score IS NOT NULL AND sub.deleted_at IS NULL
) s ON true
WHERE c.deleted_at IS NULL
ORDER BY c.active DESC, c.created_at DESC`

// GetAdminStats chạy các câu đếm cho màn thống kê admin; Teacher của từng lớp do service điền.
func (r *repository) GetAdminStats(ctx context.Context, rng httpx.StatsRange) (*dto.AdminStatsResponse, []string, error) {
	db := r.db.WithContext(ctx)
	args := map[string]any{
		"from": rng.From, "to": rng.To, "prev_from": rng.PrevFrom, "prev_to": rng.PrevTo, "tz": httpx.StatsTimeZone,
	}
	resp := &dto.AdminStatsResponse{}

	var classrooms struct {
		Current, Previous, Archived, Teachers int64
	}
	var members struct {
		Current, Previous, Pending int64
	}
	var assignments struct {
		Total, Open int64
	}
	var submissions struct {
		Current, Previous, OnTime   int64
		CurrentScore, PreviousScore *float64
	}
	var buckets []struct {
		Bucket int
		Count  int64
	}
	var rooms dto.RoomStats
	var rows []struct {
		ID                                 uint
		Name, Code, TeacherID              string
		Active                             bool
		CreatedAt                          time.Time
		Students, Assignments, Submissions int64
		AverageScore                       *float64
	}
	thptArgs := map[string]any{"type": "THPT"}
	toeicArgs := map[string]any{"type": "TOEIC"}
	for k, v := range args {
		thptArgs[k], toeicArgs[k] = v, v
	}

	queries := []struct {
		sql  string
		args map[string]any
		dst  any
	}{
		{statsClassroomsSQL, args, &classrooms},
		{statsMembersSQL, args, &members},
		{statsAssignmentsSQL, args, &assignments},
		{statsSubmissionsSQL, args, &submissions},
		{statsScoreBucketsSQL, args, &buckets},
		{statsNotSubmittedSQL, args, &resp.Assignments.NotSubmitted},
		{statsAttemptTotalsSQL, args, &resp.Attempts.Total},
		{statsAttemptModesSQL, args, &resp.Attempts.ByMode},
		{statsAttemptDailySQL, args, &resp.Attempts.Daily},
		{statsExamAveragesSQL, thptArgs, &resp.Attempts.THPT},
		{statsExamAveragesSQL, toeicArgs, &resp.Attempts.TOEIC},
		{statsViolationsSQL, args, &resp.Integrity.ByType},
		{statsTerminatedSQL, args, &resp.Integrity.Terminated},
		{statsRoomsSQL, args, &rooms},
		{statsClassroomListSQL, args, &rows},
	}
	for _, q := range queries {
		// Raw báo lỗi khi truyền map cho câu không có tham số @.
		values := []any{q.args}
		if !strings.Contains(q.sql, "@") {
			values = nil
		}
		if err := db.Raw(q.sql, values...).Scan(q.dst).Error; err != nil {
			return nil, nil, err
		}
	}

	resp.Classrooms = dto.ClassroomStats{
		Active:          httpx.PeriodCount{Current: classrooms.Current, Previous: classrooms.Previous},
		Archived:        classrooms.Archived,
		Teachers:        classrooms.Teachers,
		Students:        httpx.PeriodCount{Current: members.Current, Previous: members.Previous},
		PendingRequests: members.Pending,
	}
	resp.Assignments.Total = assignments.Total
	resp.Assignments.Open = assignments.Open
	resp.Assignments.Submissions = httpx.PeriodCount{Current: submissions.Current, Previous: submissions.Previous}
	resp.Assignments.OnTime = submissions.OnTime
	resp.Assignments.Late = submissions.Current - submissions.OnTime
	resp.Assignments.AverageScore = dto.PeriodScore{Current: submissions.CurrentScore, Previous: submissions.PreviousScore}
	resp.Assignments.ScoreDistribution = make([]int64, 10)
	for _, b := range buckets {
		resp.Assignments.ScoreDistribution[b.Bucket] = b.Count
	}
	resp.Rooms = rooms

	resp.List = make([]*dto.ClassroomStatsRow, len(rows))
	teacherIDs := make([]string, len(rows))
	for i, row := range rows {
		resp.List[i] = &dto.ClassroomStatsRow{
			ID: row.ID, Name: row.Name, Code: row.Code, Students: row.Students, Assignments: row.Assignments,
			Submissions: row.Submissions, AverageScore: row.AverageScore, Active: row.Active, CreatedAt: row.CreatedAt,
		}
		teacherIDs[i] = row.TeacherID
	}
	return resp, teacherIDs, nil
}
