//go:build integration

package integration_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"ego/services/learning/config"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"
	"ego/services/learning/internal/service"
)

func TestRoomLifecycleAndPermissions(t *testing.T) {
	db := learningTestDB(t)

	ctx := context.Background()
	classroom := &model.Classroom{Code: "ROOM001", Name: "Online class", TeacherID: "teacher", Active: true}
	if err := db.Create(classroom).Error; err != nil {
		t.Fatal(err)
	}
	approved := true
	joinedAt := time.Now()
	if err := db.Create(&model.Member{ClassroomID: classroom.ID, UserID: "student", Approved: &approved, JoinedAt: &joinedAt}).Error; err != nil {
		t.Fatal(err)
	}

	repo := repository.NewRepository(db)
	mediaClient := &fakeRoomMedia{created: map[string]bool{}, participants: map[string][]string{}}
	svc := service.New(repo, &config.AppConfig{LiveKitPublicURL: "ws://localhost:7880"}, nil, testIdentityClient{}, nil, mediaClient, nil, nil)

	now := time.Now()
	tooEarly, err := svc.CreateSchedule(ctx, "teacher", classroom.ID, dto.CreateScheduleRequest{Title: "Later", StartsAt: now.Add(time.Hour), EndsAt: now.Add(2 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.StartRoom(ctx, "teacher", tooEarly.Room.ID); !errors.Is(err, service.ErrRoomTooEarly) {
		t.Fatalf("expected ErrRoomTooEarly, got %v", err)
	}

	created, err := svc.CreateSchedule(ctx, "teacher", classroom.ID, dto.CreateScheduleRequest{Title: "Live lesson", StartsAt: now.Add(10 * time.Minute), EndsAt: now.Add(time.Hour)})
	if err != nil || created.Room == nil || created.Room.Status != model.RoomStatusScheduled {
		t.Fatalf("create schedule with room: response=%#v err=%v", created, err)
	}
	roomID := created.Room.ID
	if _, err := svc.JoinRoom(ctx, "student", roomID); !errors.Is(err, service.ErrRoomNotLive) {
		t.Fatalf("expected scheduled room to reject join, got %v", err)
	}
	if _, err := svc.JoinRoom(ctx, "outsider", roomID); !errors.Is(err, service.ErrClassroomForbidden) {
		t.Fatalf("expected outsider to be forbidden, got %v", err)
	}
	started, err := svc.StartRoom(ctx, "teacher", roomID)
	if err != nil || started.Status != model.RoomStatusLive || started.StartedAt == nil {
		t.Fatalf("start room: response=%#v err=%v", started, err)
	}
	if repeated, err := svc.StartRoom(ctx, "teacher", roomID); err != nil || repeated.Status != model.RoomStatusLive || mediaClient.createCalls != 1 {
		t.Fatalf("repeated start must be idempotent: response=%#v calls=%d err=%v", repeated, mediaClient.createCalls, err)
	}
	if _, err := svc.ArchiveClassroom(ctx, "teacher", classroom.ID); !errors.Is(err, service.ErrRoomLive) {
		t.Fatalf("expected live room to block archive, got %v", err)
	}
	connection, err := svc.JoinRoom(ctx, "student", roomID)
	if err != nil || connection.Token == "" || connection.URL != "ws://localhost:7880" {
		t.Fatalf("join room: response=%#v err=%v", connection, err)
	}
	state, repoErr := repo.GetOrCreateRoomParticipantState(ctx, roomID, "student")
	if repoErr != nil || !state.MicrophoneAllowed || !state.CameraAllowed {
		t.Fatalf("default participant state = %#v, err=%v", state, repoErr)
	}

	roomName := "classroom-room-" + fmt.Sprint(roomID)
	joined := now.Add(15 * time.Minute)
	if err := svc.HandleParticipantJoined(ctx, roomName, "student", joined); err != nil {
		t.Fatal(err)
	}
	if err := svc.HandleParticipantJoined(ctx, roomName, "student", joined.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	var openCount int64
	if err := db.Model(&model.RoomParticipant{}).Where("room_id = ? AND user_id = ? AND left_at IS NULL", roomID, "student").Count(&openCount).Error; err != nil || openCount != 1 {
		t.Fatalf("expected one idempotent open session, count=%d err=%v", openCount, err)
	}
	left := joined.Add(5 * time.Minute)
	if err := svc.HandleParticipantLeft(ctx, roomName, "student", left); err != nil {
		t.Fatal(err)
	}
	if err := svc.HandleParticipantLeft(ctx, roomName, "student", left.Add(time.Second)); err != nil {
		t.Fatalf("repeated leave must be idempotent: %v", err)
	}
	ended, err := svc.EndRoom(ctx, "teacher", roomID)
	if err != nil || ended.Status != model.RoomStatusEnded || ended.EndedAt == nil {
		t.Fatalf("end room: response=%#v err=%v", ended, err)
	}
	if repeated, err := svc.EndRoom(ctx, "teacher", roomID); err != nil || repeated.Status != model.RoomStatusEnded || mediaClient.deleteCalls != 1 {
		t.Fatalf("repeated end must be idempotent: response=%#v calls=%d err=%v", repeated, mediaClient.deleteCalls, err)
	}
	if _, err := svc.JoinRoom(ctx, "student", roomID); !errors.Is(err, service.ErrRoomEnded) {
		t.Fatalf("expected ended room to reject join, got %v", err)
	}
	if archived, err := svc.ArchiveClassroom(ctx, "teacher", classroom.ID); err != nil || archived.Active {
		t.Fatalf("expected archive after room ended, response=%#v err=%v", archived, err)
	}
}

func TestCreateScheduleAndRoomAreAtomic(t *testing.T) {
	db := learningTestDB(t)

	classroom := &model.Classroom{Code: "ATOMIC01", Name: "Atomic class", TeacherID: "teacher", Active: true}
	if err := db.Create(classroom).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`
		CREATE FUNCTION reject_room_insert() RETURNS trigger AS $$
		BEGIN RAISE EXCEPTION 'forced room insert failure'; END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER reject_room_insert BEFORE INSERT ON rooms
		FOR EACH ROW EXECUTE FUNCTION reject_room_insert();
	`).Error; err != nil {
		t.Fatal(err)
	}

	repo := repository.NewRepository(db)
	svc := service.New(repo, &config.AppConfig{}, nil, testIdentityClient{}, nil, nil, nil, nil)
	now := time.Now()
	_, err := svc.CreateSchedule(context.Background(), "teacher", classroom.ID, dto.CreateScheduleRequest{
		Title: "Must roll back", StartsAt: now.Add(time.Hour), EndsAt: now.Add(2 * time.Hour),
	})
	if err == nil {
		t.Fatal("expected forced room insert failure")
	}
	var schedules, rooms int64
	if err := db.Model(&model.Schedule{}).Where("classroom_id = ?", classroom.ID).Count(&schedules).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.Room{}).Count(&rooms).Error; err != nil {
		t.Fatal(err)
	}
	if schedules != 0 || rooms != 0 {
		t.Fatalf("transaction left partial data: schedules=%d rooms=%d", schedules, rooms)
	}
}

func TestRoomActivityPermissionsAndCleanup(t *testing.T) {
	db := learningTestDB(t)

	ctx, now := context.Background(), time.Now()
	classroom := &model.Classroom{Code: "ACTIVE01", Name: "Activity class", TeacherID: "teacher", Active: true}
	if err := db.Create(classroom).Error; err != nil {
		t.Fatal(err)
	}
	approved, joinedAt := true, now
	if err := db.Create(&model.Member{ClassroomID: classroom.ID, UserID: "student", Approved: &approved, JoinedAt: &joinedAt}).Error; err != nil {
		t.Fatal(err)
	}

	repo := repository.NewRepository(db)
	mediaClient := &fakeRoomMedia{created: map[string]bool{}, participants: map[string][]string{}}
	svc := service.New(repo, &config.AppConfig{}, nil, testIdentityClient{}, nil, mediaClient, nil, nil)
	created, err := svc.CreateSchedule(ctx, "teacher", classroom.ID, dto.CreateScheduleRequest{
		Title: "Activity", StartsAt: now.Add(-time.Minute), EndsAt: now.Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	roomID := created.Room.ID
	if _, err := svc.StartRoom(ctx, "teacher", roomID); err != nil {
		t.Fatal(err)
	}

	handRaisedAt := now
	approvedAt := now.Add(time.Second)
	studentState, repoErr := repo.GetOrCreateRoomParticipantState(ctx, roomID, "student")
	if repoErr != nil {
		t.Fatal(repoErr)
	}
	studentState.HandRaisedAt = &handRaisedAt
	studentState.MicrophoneAllowed = false
	studentState.ScreenShareRequestedAt = &handRaisedAt
	studentState.ScreenShareApprovedAt = &approvedAt
	if err := repo.UpdateRoomParticipantState(ctx, studentState, repository.ParticipantColHandRaisedAt, repository.ParticipantColMicrophone,
		repository.ParticipantColShareRequestedAt, repository.ParticipantColShareApprovedAt); err != nil {
		t.Fatal(err)
	}
	otherState, repoErr := repo.GetOrCreateRoomParticipantState(ctx, roomID, "another-student")
	if repoErr != nil {
		t.Fatal(repoErr)
	}
	otherState.HandRaisedAt = &handRaisedAt
	if err := repo.UpdateRoomParticipantState(ctx, otherState, repository.ParticipantColHandRaisedAt); err != nil {
		t.Fatal(err)
	}
	audit := &model.RoomModerationAudit{
		RoomID: roomID, ActorUserID: "teacher", TargetUserID: "student", Action: model.RoomModerationAuditActionMute,
	}
	if err := repo.CreateRoomModerationAudit(ctx, audit); err != nil {
		t.Fatal(err)
	}
	if audits, err := repo.GetRoomModerationAudits(ctx, roomID); err != nil || len(audits) != 1 {
		t.Fatalf("moderation audits = %#v, err=%v", audits, err)
	}

	hostActivity, err := svc.GetRoomActivity(ctx, "teacher", classroom.ID, roomID)
	if err != nil || !hostActivity.IsHost || len(hostActivity.Participants) != 2 {
		t.Fatalf("host activity = %#v, err=%v", hostActivity, err)
	}
	studentActivity, err := svc.GetRoomActivity(ctx, "student", classroom.ID, roomID)
	if err != nil || studentActivity.IsHost || len(studentActivity.Participants) != 1 || studentActivity.Participants[0].UserID != "student" {
		t.Fatalf("student activity = %#v, err=%v", studentActivity, err)
	}
	if studentActivity.Participants[0].MicrophoneAllowed || !studentActivity.Participants[0].CameraAllowed || studentActivity.Participants[0].ScreenShareApprovedAt == nil {
		t.Fatalf("unexpected student permissions: %#v", studentActivity.Participants[0])
	}
	if _, err := svc.GetRoomActivity(ctx, "outsider", classroom.ID, roomID); !errors.Is(err, service.ErrClassroomForbidden) {
		t.Fatalf("outsider activity error = %v, want forbidden", err)
	}
	if _, err := svc.GetRoomActivity(ctx, "teacher", classroom.ID+1, roomID); !errors.Is(err, service.ErrClassroomForbidden) {
		t.Fatalf("mismatched classroom error = %v, want forbidden", err)
	}

	if _, err := svc.EndRoom(ctx, "teacher", roomID); err != nil {
		t.Fatal(err)
	}
	var stateCount, auditCount int64
	if err := db.Model(&model.RoomParticipantState{}).Where("room_id = ?", roomID).Count(&stateCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.RoomModerationAudit{}).Where("room_id = ?", roomID).Count(&auditCount).Error; err != nil {
		t.Fatal(err)
	}
	if stateCount != 0 || auditCount != 1 {
		t.Fatalf("cleanup counts: states=%d audits=%d", stateCount, auditCount)
	}
	if _, err := svc.GetRoomActivity(ctx, "student", classroom.ID, roomID); !errors.Is(err, service.ErrRoomEnded) {
		t.Fatalf("ended room activity error = %v, want ended", err)
	}
}

func TestRoomPermissionsForMembershipAndArchivedClassroom(t *testing.T) {
	db := learningTestDB(t)

	ctx, now := context.Background(), time.Now()
	classroom := &model.Classroom{Code: "PERM0001", Name: "Permission class", TeacherID: "teacher", Active: true}
	if err := db.Create(classroom).Error; err != nil {
		t.Fatal(err)
	}
	approved, rejected, joinedAt := true, false, now
	members := []*model.Member{
		{ClassroomID: classroom.ID, UserID: "approved", Approved: &approved, JoinedAt: &joinedAt},
		{ClassroomID: classroom.ID, UserID: "pending", Approved: nil},
		{ClassroomID: classroom.ID, UserID: "rejected", Approved: &rejected},
	}
	if err := db.Create(members).Error; err != nil {
		t.Fatal(err)
	}
	schedule := &model.Schedule{ClassroomID: classroom.ID, Title: "Live", StartsAt: now.Add(-time.Minute), EndsAt: now.Add(time.Hour)}
	if err := db.Create(schedule).Error; err != nil {
		t.Fatal(err)
	}
	startedAt := now
	room := &model.Room{ScheduleID: schedule.ID, Status: model.RoomStatusLive, StartedAt: &startedAt}
	if err := db.Create(room).Error; err != nil {
		t.Fatal(err)
	}

	repo := repository.NewRepository(db)
	mediaClient := &fakeRoomMedia{created: map[string]bool{}, participants: map[string][]string{}}
	svc := service.New(repo, &config.AppConfig{LiveKitPublicURL: "ws://localhost:7880"}, nil, testIdentityClient{}, nil, mediaClient, nil, nil)

	if _, err := svc.JoinRoom(ctx, "approved", room.ID); err != nil {
		t.Fatalf("approved member could not join: %v", err)
	}
	for _, userID := range []string{"pending", "rejected", "removed"} {
		if _, err := svc.GetRoom(ctx, userID, room.ID); !errors.Is(err, service.ErrClassroomForbidden) {
			t.Fatalf("GetRoom(%q) error = %v, want forbidden", userID, err)
		}
		if _, err := svc.JoinRoom(ctx, userID, room.ID); !errors.Is(err, service.ErrClassroomForbidden) {
			t.Fatalf("JoinRoom(%q) error = %v, want forbidden", userID, err)
		}
	}
	if _, err := svc.StartRoom(ctx, "approved", room.ID); !errors.Is(err, service.ErrClassroomForbidden) {
		t.Fatalf("member start error = %v, want forbidden", err)
	}
	if _, err := svc.EndRoom(ctx, "approved", room.ID); !errors.Is(err, service.ErrClassroomForbidden) {
		t.Fatalf("member end error = %v, want forbidden", err)
	}

	classroom.Active = false
	if err := db.Save(classroom).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.JoinRoom(ctx, "approved", room.ID); !errors.Is(err, service.ErrClassroomNotActive) {
		t.Fatalf("archived classroom join error = %v, want inactive", err)
	}
}

// Giáo viên kết thúc phòng trong lúc worker đang đối chiếu với LiveKit: worker không được
// ghi lại bản room đã nạp từ trước (ENDED thành LIVE).
// Giáo viên rời phòng (webhook left ghi thời điểm vắng mặt) trong lúc worker đang đối chiếu:
// worker không được xóa thời điểm đó, phòng vẫn tự kết thúc đúng hạn.
