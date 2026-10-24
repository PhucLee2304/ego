//go:build integration

package integration_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/model"
)

// Học sinh làm đúng 1/2 câu; gradebook theo assignment, cả lớp, progress, analytics
// và reconcile đều đọc thẳng từ learning-db.
func TestGradebookAnalyticsAndReconcile(t *testing.T) {
	f, gradebook := newAssignmentAttemptFixture(t)
	ctx := context.Background()
	assignment := f.create(t, f.classroom.ID).Assignments[0]

	editable, httpErr := f.svc.GetEditableAssignment(ctx, "teacher-1", f.classroom.ID, assignment.ID)
	if httpErr != nil {
		t.Fatal(httpErr)
	}
	attempt, httpErr := f.svc.CreateAttempt(ctx, "student-1", assignment.ExamID, assignmentAttemptRequest(assignment.ID))
	if httpErr != nil {
		t.Fatal(httpErr)
	}
	// Câu 1 chọn đáp án đúng (index 0), câu 2 chọn sai (index 1).
	answers := []dto.SubmitAttemptAnswer{
		{QuestionID: editable.Questions[0].ID, SelectedOptionID: &editable.Questions[0].OptionIDs[0]},
		{QuestionID: editable.Questions[1].ID, SelectedOptionID: &editable.Questions[1].OptionIDs[1]},
	}
	result, httpErr := f.svc.SubmitAttempt(ctx, "student-1", attempt.ID, dto.SubmitAttemptRequest{Answers: answers})
	if httpErr != nil {
		t.Fatal(httpErr)
	}
	score := *result.Score

	book, httpErr := f.svc.GetGradebook(ctx, "teacher-1", f.classroom.ID, assignment.ID)
	if httpErr != nil {
		t.Fatal(httpErr)
	}
	if len(book.Submissions) != 1 || book.Submissions[0].Status != model.SubmissionStatusSubmitted || book.Submissions[0].Score == nil || *book.Submissions[0].Score != score {
		t.Fatalf("assignment gradebook = %#v", book.Submissions)
	}

	class, httpErr := f.svc.GetClassGradebook(ctx, "teacher-1", f.classroom.ID, dto.GradebookRangeQuery{})
	if httpErr != nil {
		t.Fatal(httpErr)
	}
	if len(class.Assignments) != 1 || len(class.Students) != 1 || class.CompletionRate != 100 || class.AverageScore == nil || *class.AverageScore != score {
		t.Fatalf("class gradebook = %#v", class)
	}
	from := f.opensAt.Add(48*time.Hour + time.Second)
	if empty, httpErr := f.svc.GetClassGradebook(ctx, "teacher-1", f.classroom.ID, dto.GradebookRangeQuery{From: &from}); httpErr != nil || len(empty.Assignments) != 0 {
		t.Fatalf("range must filter by due time: %#v err=%v", empty, httpErr)
	}

	progress, httpErr := f.svc.GetStudentProgress(ctx, "student-1", f.classroom.ID, "student-1")
	if httpErr != nil || progress.Student.ID != "student-1" || len(progress.Assignments) != 1 || progress.CompletionRate != 100 {
		t.Fatalf("student progress = %#v err=%v", progress, httpErr)
	}
	if _, httpErr := f.svc.GetStudentProgress(ctx, "student-2", f.classroom.ID, "student-1"); httpErr == nil || httpErr.StatusCode != http.StatusForbidden {
		t.Fatalf("other user must be 403, got %v", httpErr)
	}

	analytics, httpErr := f.svc.GetAssignmentAnalytics(ctx, "teacher-1", f.classroom.ID, assignment.ID)
	if httpErr != nil {
		t.Fatal(httpErr)
	}
	if analytics.TotalStudents != 1 || analytics.Submitted != 1 || len(analytics.Questions) != 2 ||
		analytics.Questions[0].AnsweredCount != 1 || analytics.Questions[0].WrongCount != 0 ||
		analytics.Questions[1].WrongCount != 1 || analytics.Questions[1].WrongRate != 100 {
		t.Fatalf("analytics = %#v questions=%#v %#v", analytics, analytics.Questions[0], analytics.Questions[1])
	}

	if count, httpErr := f.svc.ReconcileAssignmentSubmissions(ctx, "teacher-1", f.classroom.ID, assignment.ID); httpErr != nil || count != 0 {
		t.Fatalf("synced gradebook must reconcile nothing: count=%d err=%v", count, httpErr)
	}
	// Làm lệch submission bằng tay; reconcile sửa lại theo attempt và push GRADEBOOK_UPDATED.
	f.db.Model(&model.AssignmentSubmission{}).Where("assignment_id = ?", assignment.ID).Update("score", nil)
	before := len(gradebook.all())
	if count, httpErr := f.svc.ReconcileAssignmentSubmissions(ctx, "teacher-1", f.classroom.ID, assignment.ID); httpErr != nil || count != 1 {
		t.Fatalf("reconcile: count=%d err=%v", count, httpErr)
	}
	var submission model.AssignmentSubmission
	f.db.Where("assignment_id = ?", assignment.ID).First(&submission)
	if submission.Score == nil || *submission.Score != score || len(gradebook.all()) != before+1 {
		t.Fatalf("reconcile must restore the score: %#v events=%d", submission, len(gradebook.all())-before)
	}

	if _, httpErr := f.svc.ReconcileAssignmentSubmissions(ctx, "teacher-2", f.classroom.ID, assignment.ID); httpErr == nil || httpErr.StatusCode != http.StatusForbidden {
		t.Fatalf("other teacher must be 403, got %v", httpErr)
	}
	if _, httpErr := f.svc.GetGradebook(ctx, "teacher-2", f.classroom.ID, assignment.ID); httpErr == nil || httpErr.StatusCode != http.StatusForbidden {
		t.Fatalf("other teacher gradebook must be 403, got %v", httpErr)
	}
}
