//go:build integration

package integration_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"ego/platform/httpx"
	"ego/services/learning/config"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"
	"ego/services/learning/internal/service"

	"gorm.io/gorm"
)

type assignmentFixture struct {
	db        *gorm.DB
	svc       service.Service
	events    *attemptEvents
	classroom *model.Classroom
	other     *model.Classroom
	opensAt   time.Time
}

func newAssignmentFixture(t *testing.T) *assignmentFixture {
	t.Helper()
	db := learningTestDB(t)
	approved := true
	joinedAt := time.Now()
	classroom := &model.Classroom{Code: "ASSIGN01", Name: "Class A", TeacherID: "teacher-1", Active: true}
	other := &model.Classroom{Code: "ASSIGN02", Name: "Class B", TeacherID: "teacher-1", Active: true}
	for _, c := range []*model.Classroom{classroom, other} {
		if err := db.Create(c).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&model.Member{ClassroomID: classroom.ID, UserID: "student-1", Approved: &approved, JoinedAt: &joinedAt}).Error; err != nil {
		t.Fatal(err)
	}
	events := &attemptEvents{}
	svc := service.New(repository.NewRepository(db), &config.AppConfig{}, nil, testIdentityClient{}, nil, nil, nil, events.publish)
	return &assignmentFixture{db: db, svc: svc, events: events, classroom: classroom, other: other, opensAt: time.Now().Add(-time.Hour).UTC().Truncate(time.Second)}
}

func (f *assignmentFixture) schedule(classroomID uint) dto.AssignmentScheduleRequest {
	return dto.AssignmentScheduleRequest{ClassroomID: classroomID, Duration: 45, OpensAt: f.opensAt, DueAt: f.opensAt.Add(48 * time.Hour)}
}

func assignmentQuestion(content string) dto.CreateAssignmentQuestionRequest {
	return dto.CreateAssignmentQuestionRequest{Content: content, Options: []string{"a", "b", "c", "d"}, CorrectOptionIndex: 0}
}

func (f *assignmentFixture) create(t *testing.T, classroomIDs ...uint) *dto.CreatedAssignmentsResponse {
	t.Helper()
	schedules := make([]dto.AssignmentScheduleRequest, len(classroomIDs))
	for i, id := range classroomIDs {
		schedules[i] = f.schedule(id)
	}
	created, httpErr := f.svc.CreateAssignments(context.Background(), "teacher-1", dto.CreateAssignmentsRequest{
		Title: " Weekly quiz ", Questions: []dto.CreateAssignmentQuestionRequest{assignmentQuestion("1 + 1"), assignmentQuestion("2 + 2")}, Schedules: schedules,
	})
	if httpErr != nil {
		t.Fatal(httpErr)
	}
	return created
}

// startAssignmentAttempt ghi thẳng một attempt đang làm của assignment, không qua
// CreateAttempt (không tạo submission, không kiểm tra assignment).
func (f *assignmentFixture) startAssignmentAttempt(t *testing.T, assignmentID, examID uint, userID string) *model.Attempt {
	t.Helper()
	duration := 45
	expiresAt := time.Now().Add(45 * time.Minute)
	attempt := &model.Attempt{Mode: model.AttemptModeAssignment, Status: model.AttemptStatusInProgress, StartedAt: time.Now(), ExpiresAt: &expiresAt, Duration: &duration,
		TotalQuestions: 2, ExamID: examID, UserID: userID, ContextType: model.AttemptContextTypeClassroomAssignment, ContextID: &assignmentID}
	if err := f.db.Create(attempt).Error; err != nil {
		t.Fatal(err)
	}
	return attempt
}

// Tạo assignment: đề private + assignment cho nhiều lớp + thông báo nằm trong một
// transaction; đề không hợp lệ thì không còn gì được ghi.
func TestCreateAssignmentsIsAtomic(t *testing.T) {
	f := newAssignmentFixture(t)
	ctx := context.Background()

	created := f.create(t, f.classroom.ID, f.other.ID)
	if len(created.Assignments) != 2 || created.Assignments[0].ExamID != created.Assignments[1].ExamID || created.Assignments[0].Title != "Weekly quiz" {
		t.Fatalf("created = %#v", created)
	}
	var notifications []model.NotificationOutbox
	f.db.Order("id").Find(&notifications)
	// Lớp B chưa có học sinh nên không có thông báo (outbox bỏ qua danh sách nhận rỗng).
	if len(notifications) != 2 || notifications[0].Source != model.NotificationSourceAssignment || !strings.HasSuffix(notifications[0].EventKey, ":created") {
		t.Fatalf("notifications = %#v", notifications)
	}

	_, httpErr := f.svc.CreateAssignments(ctx, "teacher-1", dto.CreateAssignmentsRequest{
		Title: "Broken", Questions: []dto.CreateAssignmentQuestionRequest{{Content: "Q", Options: []string{"a", " ", "c", "d"}}}, Schedules: []dto.AssignmentScheduleRequest{f.schedule(f.classroom.ID)},
	})
	if httpErr == nil || httpErr.StatusCode != http.StatusBadRequest || httpErr.Message != "[ERROR] Question options cannot be blank" {
		t.Fatalf("invalid exam must be 400 with the exam message, got %v", httpErr)
	}
	var exams, assignments int64
	f.db.Model(&model.Exam{}).Count(&exams)
	f.db.Model(&model.Assignment{}).Count(&assignments)
	if exams != 1 || assignments != 2 {
		t.Fatalf("failed create must leave no orphan rows: exams=%d assignments=%d", exams, assignments)
	}

	if _, httpErr := f.svc.CreateAssignments(ctx, "teacher-2", dto.CreateAssignmentsRequest{Title: "X", Questions: []dto.CreateAssignmentQuestionRequest{assignmentQuestion("Q")}, Schedules: []dto.AssignmentScheduleRequest{f.schedule(f.classroom.ID)}}); httpErr == nil || httpErr.StatusCode != http.StatusForbidden {
		t.Fatalf("other teacher must be 403, got %v", httpErr)
	}
}

// editRequest dựng form sửa giống client: nội dung đề hiện tại, chưa có lịch nào.
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

// Các luồng sửa riêng lẻ giờ đi qua endpoint gộp UpdateAssignment.
func updateContent(ctx context.Context, svc service.Service, teacherID string, classroomID, assignmentID uint, req dto.UpdateAssignmentContentRequest) (*dto.AssignmentResponse, *httpx.HTTPError) {
	return svc.UpdateAssignment(ctx, teacherID, classroomID, assignmentID, dto.UpdateAssignmentRequest{Title: req.Title, Description: req.Description, Questions: req.Questions, Confirm: req.Confirm, Reason: req.Reason})
}

func updateSchedule(ctx context.Context, t *testing.T, svc service.Service, classroomID, assignmentID uint, req dto.UpdateAssignmentScheduleRequest) (*dto.AssignmentResponse, *httpx.HTTPError) {
	request := editRequest(ctx, t, svc, classroomID, assignmentID)
	request.Schedules = []dto.AssignmentScheduleRequest{{ClassroomID: classroomID, Duration: req.Duration, OpensAt: req.OpensAt, DueAt: req.DueAt}}
	request.Confirm = req.Confirm
	return svc.UpdateAssignment(ctx, "teacher-1", classroomID, assignmentID, request)
}

func TestAssignmentViewsAndCalendar(t *testing.T) {
	f := newAssignmentFixture(t)
	ctx := context.Background()
	created := f.create(t, f.classroom.ID)
	assignmentID := created.Assignments[0].ID

	// Lớp đã có bài thì là sửa lịch (ở đây không đổi gì), không tạo bản trùng.
	same := editRequest(ctx, t, f.svc, f.classroom.ID, assignmentID)
	same.Schedules = []dto.AssignmentScheduleRequest{f.schedule(f.classroom.ID)}
	if _, httpErr := f.svc.UpdateAssignment(ctx, "teacher-1", f.classroom.ID, assignmentID, same); httpErr != nil {
		t.Fatalf("unchanged schedule must be a no-op, got %v", httpErr)
	}
	var inClass int64
	f.db.Model(&model.Assignment{}).Where("classroom_id = ?", f.classroom.ID).Count(&inClass)
	if inClass != 1 {
		t.Fatalf("existing class must not get a duplicate assignment, got %d", inClass)
	}
	assign := editRequest(ctx, t, f.svc, f.classroom.ID, assignmentID)
	assign.Schedules = []dto.AssignmentScheduleRequest{f.schedule(f.classroom.ID), f.schedule(f.other.ID)}
	_, httpErr := f.svc.UpdateAssignment(ctx, "teacher-1", f.classroom.ID, assignmentID, assign)
	var copied model.Assignment
	if httpErr != nil || f.db.Where("classroom_id = ?", f.other.ID).First(&copied).Error != nil || copied.ExamID != created.Assignments[0].ExamID {
		t.Fatalf("assign existing = %#v err=%v", copied, httpErr)
	}

	query := dto.AssignmentsQuery{}
	query.Normalize()
	list, _, httpErr := f.svc.GetAssignments(ctx, "student-1", f.classroom.ID, query)
	if httpErr != nil || len(list) != 1 || list[0].ID != assignmentID {
		t.Fatalf("student list = %#v err=%v", list, httpErr)
	}
	if _, _, httpErr := f.svc.GetAssignments(ctx, "stranger", f.classroom.ID, query); httpErr == nil || httpErr.StatusCode != http.StatusForbidden {
		t.Fatalf("non-member must be 403, got %v", httpErr)
	}
	if got, httpErr := f.svc.GetAssignment(ctx, "teacher-1", f.classroom.ID, assignmentID); httpErr != nil || got.ID != assignmentID {
		t.Fatalf("teacher get = %#v err=%v", got, httpErr)
	}

	calendar, httpErr := f.svc.GetCalendar(ctx, "student-1", f.classroom.ID, dto.CalendarQuery{From: f.opensAt, To: f.opensAt.Add(72 * time.Hour)})
	if httpErr != nil || len(calendar) != 1 || calendar[0].CalendarType != dto.CalendarTypeAssignmentDeadline || *calendar[0].RelatedAssignmentID != assignmentID {
		t.Fatalf("calendar must include the assignment deadline: %#v err=%v", calendar, httpErr)
	}
}

// Đổi lịch khi đã có bài làm: rút ngắn cần xác nhận; xác nhận rồi thì thời hạn
// attempt đang làm đổi cùng transaction.
func TestUpdateAssignmentScheduleSyncsActiveAttempts(t *testing.T) {
	f := newAssignmentFixture(t)
	ctx := context.Background()
	created := f.create(t, f.classroom.ID)
	assignment := created.Assignments[0]
	attempt := f.startAssignmentAttempt(t, assignment.ID, assignment.ExamID, "student-1")

	shorter := dto.UpdateAssignmentScheduleRequest{Duration: 20, OpensAt: f.opensAt, DueAt: f.opensAt.Add(24 * time.Hour)}
	if _, httpErr := updateSchedule(ctx, t, f.svc, f.classroom.ID, assignment.ID, shorter); httpErr == nil || httpErr.Message != service.ErrAssignmentScheduleConfirmation.Message {
		t.Fatalf("shortening with attempts must require confirmation, got %v", httpErr)
	}
	shorter.Confirm = true
	updated, httpErr := updateSchedule(ctx, t, f.svc, f.classroom.ID, assignment.ID, shorter)
	if httpErr != nil || updated.Duration != 20 {
		t.Fatalf("update = %#v err=%v", updated, httpErr)
	}
	var after model.Attempt
	f.db.First(&after, attempt.ID)
	if *after.Duration != 20 || after.ExpiresAt.Sub(attempt.StartedAt) > 20*time.Minute+time.Second {
		t.Fatalf("active attempt must follow the new schedule: duration=%v expiresAt=%v", *after.Duration, after.ExpiresAt)
	}
}

// Tắt assignment: nộp ngay bài đang làm trong cùng transaction, push WS sau commit.
func TestDeactivateAssignmentSubmitsActiveAttempts(t *testing.T) {
	f := newAssignmentFixture(t)
	ctx := context.Background()
	created := f.create(t, f.classroom.ID)
	assignment := created.Assignments[0]
	attempt := f.startAssignmentAttempt(t, assignment.ID, assignment.ExamID, "student-1")

	resp, httpErr := f.svc.SetAssignmentActive(ctx, "teacher-1", f.classroom.ID, assignment.ID, false)
	if httpErr != nil || resp.Active {
		t.Fatalf("deactivate = %#v err=%v", resp, httpErr)
	}
	var after model.Attempt
	f.db.First(&after, attempt.ID)
	if after.Status != model.AttemptStatusSubmitted {
		t.Fatalf("active attempt must be submitted, got %s", after.Status)
	}
	if got := f.events.types(); len(got) != 2 || got[0] != "ATTEMPT_SUBMITTING" || got[1] != "ATTEMPT_SUBMITTED" {
		t.Fatalf("events = %v", got)
	}
	var reminders int64
	f.db.Model(&model.NotificationOutbox{}).Where("event_key LIKE ? AND processed_at IS NULL", "assignment:%:due-reminder%").Count(&reminders)
	if reminders != 0 {
		t.Fatalf("due reminders must be cancelled, %d left", reminders)
	}
	if _, httpErr := f.svc.SetAssignmentActive(ctx, "teacher-1", f.classroom.ID, assignment.ID, true); httpErr != nil {
		t.Fatalf("reactivate: %v", httpErr)
	}
}

// Sửa nội dung: đề chưa có bài làm thì sửa tự do; có bài làm thì phải xác nhận + lý
// do, chỉ được đính chính, chấm lại và giữ assignment tạm dừng. Đính chính sai cấu
// trúc thì rollback toàn bộ, assignment không bị kẹt ở trạng thái tạm dừng.
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
	if _, err := f.svc.UpdateAttemptAnswer(ctx, "student-1", attempt.ID, editable.Questions[0].ID, &selected); err != nil {
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
	if got := f.events.types(); len(got) != 2 || got[1] != "ATTEMPT_SUBMITTED" {
		t.Fatalf("events after commit = %v", got)
	}

	audits, httpErr := f.svc.GetAssignmentAudits(ctx, "teacher-1", f.classroom.ID, assignment.ID)
	if httpErr != nil || len(audits) != 2 || audits[0].Reason != "wrong key" || audits[0].RegradedAt == nil || len(audits[0].ScoreChanges) != 1 || audits[0].Actor == nil {
		t.Fatalf("audits = %#v err=%v", audits, httpErr)
	}
}

// Màn sửa gộp: client gửi cả form, backend tự so và ghi nội dung + lịch + lớp mới trong
// một transaction. Cần xác nhận
// (409) thì không phần nào được ghi, kể cả lớp mới; xác nhận xong thì ghi đủ cả ba.
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

// Gửi lại đúng form hiện tại thì không đổi gì: không cần xác nhận dù đã có bài làm,
// không tạo audit, bài vẫn bật.
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

// Chỉ đổi lịch (không đổi đề) vẫn ghi một dòng lịch sử.
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

func TestResetAssignmentAttempt(t *testing.T) {
	f := newAssignmentFixture(t)
	ctx := context.Background()
	created := f.create(t, f.classroom.ID)
	assignment := created.Assignments[0]
	attempt := f.startAssignmentAttempt(t, assignment.ID, assignment.ExamID, "student-1")
	if err := f.db.Create(&model.AssignmentSubmission{AssignmentID: assignment.ID, StudentID: "student-1", AttemptID: attempt.ID}).Error; err != nil {
		t.Fatal(err)
	}

	if httpErr := f.svc.ResetAssignmentAttempt(ctx, "teacher-1", f.classroom.ID, assignment.ID, "student-2"); httpErr == nil || httpErr.StatusCode != http.StatusNotFound {
		t.Fatalf("student without submission must be 404, got %v", httpErr)
	}
	if httpErr := f.svc.ResetAssignmentAttempt(ctx, "teacher-1", f.classroom.ID, assignment.ID, "student-1"); httpErr != nil {
		t.Fatal(httpErr)
	}
	var attempts, submissions int64
	f.db.Model(&model.Attempt{}).Count(&attempts)
	f.db.Model(&model.AssignmentSubmission{}).Count(&submissions)
	if attempts != 0 || submissions != 0 {
		t.Fatalf("reset must remove attempt and submission: attempts=%d submissions=%d", attempts, submissions)
	}
}
