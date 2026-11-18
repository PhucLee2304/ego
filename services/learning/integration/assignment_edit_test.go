//go:build integration

package integration_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"ego/platform/httpx"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/service"
)

func editRequest(ctx context.Context, t *testing.T, svc service.Service, classroomID, assignmentID uint) dto.UpdateAssignmentRequest {
	t.Helper()
	editable, httpErr := svc.GetEditableAssignment(ctx, "teacher-1", classroomID, assignmentID)
	if httpErr != nil {
		t.Fatal(httpErr)
	}
	questions := make([]dto.UpdateAssignmentQuestionRequest, len(editable.Questions))
	for i, q := range editable.Questions {
		explanation := ""
		if q.Explanation != nil {
			explanation = *q.Explanation
		}
		questions[i] = dto.UpdateAssignmentQuestionRequest{ID: q.ID, Content: q.Content, Explanation: &explanation, Options: q.Options, CorrectOptionIndex: q.CorrectOptionIndex, ExcludedFromScore: q.ExcludedFromScore, OptionIDs: q.OptionIDs}
	}
	return dto.UpdateAssignmentRequest{Title: editable.Title, Description: editable.Description, Questions: questions}
}

func updateContent(ctx context.Context, svc service.Service, teacherID string, classroomID, assignmentID uint, req dto.UpdateAssignmentContentRequest) (*dto.AssignmentResponse, *httpx.HTTPError) {
	return svc.UpdateAssignment(ctx, teacherID, classroomID, assignmentID, dto.UpdateAssignmentRequest{Title: req.Title, Description: req.Description, Questions: req.Questions, Confirm: req.Confirm, Reason: req.Reason})
}

func updateSchedule(ctx context.Context, t *testing.T, svc service.Service, classroomID, assignmentID uint, req dto.UpdateAssignmentScheduleRequest) (*dto.AssignmentResponse, *httpx.HTTPError) {
	request := editRequest(ctx, t, svc, classroomID, assignmentID)
	request.Schedules = []dto.AssignmentScheduleRequest{{ClassroomID: classroomID, Duration: req.Duration, OpensAt: req.OpensAt, DueAt: req.DueAt}}
	request.Confirm = req.Confirm
	return svc.UpdateAssignment(ctx, "teacher-1", classroomID, assignmentID, request)
}

func TestUpdateAssignmentContent(t *testing.T) {
	f := newAssignmentFixture(t)
	ctx := context.Background()
	created := f.create(t, f.classroom.ID, f.other.ID)
	assignment := created.Assignments[0]

	editable, httpErr := f.svc.GetEditableAssignment(ctx, "teacher-1", f.classroom.ID, assignment.ID)
	if httpErr != nil || editable.HasAttempts || len(editable.Questions) != 2 || len(editable.AffectedAssignments) != 2 {
		t.Fatalf("editable = %#v err=%v", editable, httpErr)
	}
	request := func(title string, correct int) dto.UpdateAssignmentContentRequest {
		questions := make([]dto.UpdateAssignmentQuestionRequest, len(editable.Questions))
		for i, q := range editable.Questions {
			// Giống client Flutter: luôn gửi explanation dạng chuỗi (rỗng nếu không có).
			explanation := ""
			if q.Explanation != nil {
				explanation = *q.Explanation
			}
			questions[i] = dto.UpdateAssignmentQuestionRequest{ID: q.ID, Content: q.Content, Explanation: &explanation, Options: q.Options, CorrectOptionIndex: q.CorrectOptionIndex, OptionIDs: q.OptionIDs}
		}
		questions[0].CorrectOptionIndex = correct
		return dto.UpdateAssignmentContentRequest{Title: title, Questions: questions}
	}

	// Chưa có bài làm: sửa tự do, assignment giữ trạng thái bật.
	renamed, httpErr := updateContent(ctx, f.svc, "teacher-1", f.classroom.ID, assignment.ID, request("Renamed", 0))
	if httpErr != nil || renamed.Title != "Renamed" {
		t.Fatalf("free edit = %#v err=%v", renamed, httpErr)
	}
	var active int64
	f.db.Model(&model.Assignment{}).Where("active = ? AND title = ?", true, "Renamed").Count(&active)
	if active != 2 {
		t.Fatalf("assignments must stay active and renamed, %d", active)
	}

	editable, _ = f.svc.GetEditableAssignment(ctx, "teacher-1", f.classroom.ID, assignment.ID)
	attempt := f.startAssignmentAttempt(t, assignment.ID, assignment.ExamID, "student-1")
	selected := editable.Questions[0].OptionIDs[1]
	if _, err := f.svc.UpdateAttemptAnswer(ctx, "student-1", "", attempt.ID, editable.Questions[0].ID, &selected); err != nil {
		t.Fatal(err)
	}

	if _, httpErr := updateContent(ctx, f.svc, "teacher-1", f.classroom.ID, assignment.ID, request("Renamed", 1)); httpErr == nil || httpErr.Message != service.ErrAssignmentContentConfirmation.Message {
		t.Fatalf("edit with attempts must require confirmation, got %v", httpErr)
	}
	confirmed := request("Renamed", 1)
	confirmed.Confirm = true
	if _, httpErr := updateContent(ctx, f.svc, "teacher-1", f.classroom.ID, assignment.ID, confirmed); httpErr == nil || httpErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("correction without reason must be 400, got %v", httpErr)
	}
	structural := request("New title", 1)
	structural.Confirm, structural.Reason = true, "typo"
	if _, httpErr := updateContent(ctx, f.svc, "teacher-1", f.classroom.ID, assignment.ID, structural); httpErr == nil || httpErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("structural change with attempts must be 400, got %v", httpErr)
	}
	var stillActive model.Attempt
	f.db.First(&stillActive, attempt.ID)
	f.db.Model(&model.Assignment{}).Where("active = ?", true).Count(&active)
	if stillActive.Status != model.AttemptStatusInProgress || active != 2 || len(f.events.types()) != 0 {
		t.Fatalf("rejected correction must roll back everything: attempt=%s active=%d events=%v", stillActive.Status, active, f.events.types())
	}

	confirmed.Reason = "wrong key"
	_, httpErr = updateContent(ctx, f.svc, "teacher-1", f.classroom.ID, assignment.ID, confirmed)
	corrected, _ := f.svc.GetEditableAssignment(ctx, "teacher-1", f.classroom.ID, assignment.ID)
	if httpErr != nil || corrected.Questions[0].CorrectOptionIndex != 1 {
		t.Fatalf("correction = %#v err=%v", corrected, httpErr)
	}
	var regraded model.Attempt
	f.db.First(&regraded, attempt.ID)
	f.db.Model(&model.Assignment{}).Where("active = ?", true).Count(&active)
	if regraded.Status != model.AttemptStatusSubmitted || regraded.Score == nil || *regraded.Score != 5 || active != 0 {
		t.Fatalf("attempt must be submitted and regraded, assignments paused: status=%s score=%v active=%d", regraded.Status, regraded.Score, active)
	}
	if got := f.events.types(); len(got) != 1 || got[0] != "ATTEMPT_SUBMITTED" {
		t.Fatalf("events after commit = %v", got)
	}

	audits, httpErr := f.svc.GetAssignmentAudits(ctx, "teacher-1", f.classroom.ID, assignment.ID)
	if httpErr != nil || len(audits) != 2 || audits[0].Reason != "wrong key" || audits[0].RegradedAt == nil || len(audits[0].ScoreChanges) != 1 || audits[0].Actor == nil {
		t.Fatalf("audits = %#v err=%v", audits, httpErr)
	}
}

func TestUpdateAssignmentCombined(t *testing.T) {
	f := newAssignmentFixture(t)
	ctx := context.Background()
	third := &model.Classroom{Code: "ASSIGN03", Name: "Class C", TeacherID: "teacher-1", Active: true}
	if err := f.db.Create(third).Error; err != nil {
		t.Fatal(err)
	}
	assignment := f.create(t, f.classroom.ID).Assignments[0]
	form := func(title string, correct int, schedules ...dto.AssignmentScheduleRequest) dto.UpdateAssignmentRequest {
		request := editRequest(ctx, t, f.svc, f.classroom.ID, assignment.ID)
		request.Title = title
		request.Questions[0].CorrectOptionIndex = correct
		request.Schedules = schedules
		return request
	}
	countIn := func(classroomID uint) int64 {
		var n int64
		f.db.Model(&model.Assignment{}).Where("classroom_id = ?", classroomID).Count(&n)
		return n
	}

	// Chưa có bài làm: đổi tên, kéo dài lịch, giao thêm lớp B; lớp B nhận tiêu đề mới.
	longer := dto.AssignmentScheduleRequest{ClassroomID: f.classroom.ID, Duration: 60, OpensAt: f.opensAt, DueAt: f.opensAt.Add(72 * time.Hour)}
	updated, httpErr := f.svc.UpdateAssignment(ctx, "teacher-1", f.classroom.ID, assignment.ID, form("Renamed", 0, longer, f.schedule(f.other.ID)))
	if httpErr != nil || updated.Title != "Renamed" || updated.Duration != 60 {
		t.Fatalf("combined update = %#v err=%v", updated, httpErr)
	}
	var copied model.Assignment
	if err := f.db.Where("classroom_id = ?", f.other.ID).First(&copied).Error; err != nil || copied.Title != "Renamed" {
		t.Fatalf("new class must get the renamed assignment: %#v err=%v", copied, err)
	}
	audits, _ := f.svc.GetAssignmentAudits(ctx, "teacher-1", f.classroom.ID, assignment.ID)
	if len(audits) != 1 || !hasChange(audits[0].Changes, "Title: \"") || !hasChange(audits[0].Changes, ": due ") || !hasChange(audits[0].Changes, "Assigned to "+f.other.Name) {
		t.Fatalf("edit without attempts must be audited with its changes: %#v", audits)
	}

	// Có bài làm: đính chính đáp án + giao lớp C khi chưa xác nhận thì 409, lớp C không có gì.
	f.startAssignmentAttempt(t, assignment.ID, assignment.ExamID, "student-1")
	request := form("Renamed", 1, longer, f.schedule(third.ID))
	if _, httpErr := f.svc.UpdateAssignment(ctx, "teacher-1", f.classroom.ID, assignment.ID, request); httpErr == nil || httpErr.StatusCode != http.StatusConflict {
		t.Fatalf("correction with attempts must require confirmation, got %v", httpErr)
	}
	if n := countIn(third.ID); n != 0 {
		t.Fatalf("rejected update must not assign the new class, got %d", n)
	}
	request.Confirm, request.Reason = true, "wrong key"
	if _, httpErr := f.svc.UpdateAssignment(ctx, "teacher-1", f.classroom.ID, assignment.ID, request); httpErr != nil {
		t.Fatalf("confirmed update: %v", httpErr)
	}
	if n := countIn(third.ID); n != 1 {
		t.Fatalf("confirmed update must assign the new class, got %d", n)
	}
	audits, _ = f.svc.GetAssignmentAudits(ctx, "teacher-1", f.classroom.ID, assignment.ID)
	if len(audits) != 2 || audits[0].Reason != "wrong key" || !hasChange(audits[0].Changes, "Q1: correct answer A → B") || !hasChange(audits[0].Changes, "Assigned to Class C") {
		t.Fatalf("correction must be audited with the shared reason: %#v", audits)
	}
}

func TestUpdateAssignmentUnchangedIsNoop(t *testing.T) {
	f := newAssignmentFixture(t)
	ctx := context.Background()
	assignment := f.create(t, f.classroom.ID).Assignments[0]
	f.startAssignmentAttempt(t, assignment.ID, assignment.ExamID, "student-1")
	request := editRequest(ctx, t, f.svc, f.classroom.ID, assignment.ID)
	request.Schedules = []dto.AssignmentScheduleRequest{f.schedule(f.classroom.ID)}
	updated, httpErr := f.svc.UpdateAssignment(ctx, "teacher-1", f.classroom.ID, assignment.ID, request)
	if httpErr != nil || !updated.Active {
		t.Fatalf("unchanged form must save without confirmation: %#v err=%v", updated, httpErr)
	}
	audits, _ := f.svc.GetAssignmentAudits(ctx, "teacher-1", f.classroom.ID, assignment.ID)
	if len(audits) != 0 {
		t.Fatalf("unchanged form must not add an audit, got %d", len(audits))
	}
}

func TestUpdateAssignmentScheduleOnlyIsAudited(t *testing.T) {
	f := newAssignmentFixture(t)
	ctx := context.Background()
	assignment := f.create(t, f.classroom.ID).Assignments[0]
	request := editRequest(ctx, t, f.svc, f.classroom.ID, assignment.ID)
	schedule := f.schedule(f.classroom.ID)
	schedule.Duration += 15
	request.Schedules = []dto.AssignmentScheduleRequest{schedule}
	if _, httpErr := f.svc.UpdateAssignment(ctx, "teacher-1", f.classroom.ID, assignment.ID, request); httpErr != nil {
		t.Fatalf("schedule update: %v", httpErr)
	}
	audits, _ := f.svc.GetAssignmentAudits(ctx, "teacher-1", f.classroom.ID, assignment.ID)
	if len(audits) != 1 || len(audits[0].Changes) != 1 || !hasChange(audits[0].Changes, "time limit") {
		t.Fatalf("schedule-only edit must be audited: %#v", audits)
	}
}

func hasChange(changes []string, part string) bool {
	for _, change := range changes {
		if strings.Contains(change, part) {
			return true
		}
	}
	return false
}
