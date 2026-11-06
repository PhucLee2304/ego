//go:build integration

package integration_test

import (
	"context"
	"testing"
	"time"

	"ego/platform/httpx"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"
	"ego/services/learning/internal/service"
)

// GetAdminStats đếm đúng lớp, thành viên, bài nộp, lượt làm bài, vi phạm và bảng lớp trong kỳ.
func TestGetAdminStats(t *testing.T) {
	db := learningTestDB(t)
	ctx := context.Background()
	now := time.Now()
	yes := true
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}

	active := &model.Classroom{Code: "STAT0001", Name: "Active class", TeacherID: "teacher-1", Active: true}
	archived := &model.Classroom{Code: "STAT0002", Name: "Archived class", TeacherID: "teacher-2", Active: true}
	must(db.Create(active).Error)
	must(db.Create(archived).Error)
	must(db.Model(archived).Update("active", false).Error)
	must(db.Create(&[]model.Member{
		{ClassroomID: active.ID, UserID: "s1", Approved: &yes, ApprovedAt: &now},
		{ClassroomID: active.ID, UserID: "s2", Approved: &yes, ApprovedAt: &now},
		{ClassroomID: active.ID, UserID: "s3"},
	}).Error)

	assignmentExam := &model.Exam{Title: "Class quiz", Type: model.ExamTypeAssignment}
	thpt := &model.Exam{Title: "THPT 2025", Type: model.ExamTypeTHPT, IsPublic: true}
	must(db.Create(assignmentExam).Error)
	must(db.Create(thpt).Error)
	assignment := &model.Assignment{Title: "Quiz", ExamID: assignmentExam.ID, ClassroomID: active.ID, Duration: 30,
		OpensAt: now.Add(-24 * time.Hour), DueAt: now.Add(24 * time.Hour), Active: true}
	must(db.Create(assignment).Error)

	submitted := func(examID uint, user string, mode model.AttemptMode, ctxType model.AttemptContextType, score float64) *model.Attempt {
		at := now.Add(-time.Hour)
		a := &model.Attempt{ExamID: examID, UserID: user, Mode: mode, Status: model.AttemptStatusSubmitted, ContextType: ctxType,
			StartedAt: at.Add(-time.Hour), SubmittedAt: &at, Score: &score}
		must(db.Create(a).Error)
		return a
	}
	for i, s := range []struct {
		user  string
		score float64
	}{{"s1", 8}, {"s2", 5.5}} {
		attempt := submitted(assignmentExam.ID, s.user, model.AttemptModeAssignment, model.AttemptContextTypeClassroomAssignment, s.score)
		score := s.score
		must(db.Create(&model.AssignmentSubmission{AssignmentID: assignment.ID, StudentID: s.user, AttemptID: attempt.ID, Score: &score}).Error)
		if i == 0 {
			must(db.Create(&model.AttemptIntegrityEvent{AttemptID: attempt.ID, EventID: "e1", Type: model.IntegrityEventAwayStart,
				Level: model.IntegrityLevelViolation, ReceivedAt: now.Add(-90 * time.Minute)}).Error)
		}
	}
	submitted(thpt.ID, "s1", model.AttemptModePractice, model.AttemptContextTypeStandalone, 7)

	today := now.In(httpx.StatsLocation).Format(time.DateOnly)
	from := now.In(httpx.StatsLocation).AddDate(0, 0, -6).Format(time.DateOnly)
	svc := service.New(repository.NewRepository(db), nil, nil, testIdentityClient{}, nil, nil, nil, nil)
	resp, httpErr := svc.GetAdminStats(ctx, httpx.StatsQuery{From: from, To: today})
	if httpErr != nil {
		t.Fatalf("%v: %v", httpErr, httpErr.Unwrap())
	}

	c := resp.Classrooms
	if c.Active.Current != 1 || c.Archived != 1 || c.Teachers != 1 || c.Students.Current != 2 || c.PendingRequests != 1 {
		t.Fatalf("classrooms = %+v", c)
	}
	a := resp.Assignments
	if a.Total != 1 || a.Open != 1 || a.Submissions.Current != 2 || a.OnTime != 2 || a.Late != 0 || a.NotSubmitted != 0 {
		t.Fatalf("assignments = %+v", a)
	}
	if a.AverageScore.Current == nil || *a.AverageScore.Current != 6.75 || a.AverageScore.Previous != nil {
		t.Fatalf("average score = %+v", a.AverageScore)
	}
	if len(a.ScoreDistribution) != 10 || a.ScoreDistribution[8] != 1 || a.ScoreDistribution[5] != 1 {
		t.Fatalf("distribution = %v", a.ScoreDistribution)
	}
	at := resp.Attempts
	if at.Total.Current != 3 || len(at.Daily) == 0 || len(at.THPT) != 1 || at.THPT[0].AverageScore != 7 || len(at.TOEIC) != 0 {
		t.Fatalf("attempts = %+v thpt=%+v", at, at.THPT)
	}
	if len(resp.Integrity.ByType) != 1 || resp.Integrity.ByType[0].Count != 1 {
		t.Fatalf("integrity = %+v", resp.Integrity)
	}
	if len(resp.List) != 2 || resp.List[0].ID != active.ID || resp.List[0].Students != 2 || resp.List[0].Submissions != 2 ||
		resp.List[0].Teacher == nil || resp.List[0].Teacher.Name != "teacher-1" || resp.List[1].AverageScore != nil {
		t.Fatalf("list = %+v %+v", resp.List[0], resp.List[1])
	}

	all, httpErr := svc.GetAdminStats(ctx, httpx.StatsQuery{})
	if httpErr != nil || all.Attempts.Total.Current != 3 || all.Attempts.Total.Previous != 0 {
		t.Fatalf("all time = %+v err=%v", all.Attempts.Total, httpErr)
	}
}
