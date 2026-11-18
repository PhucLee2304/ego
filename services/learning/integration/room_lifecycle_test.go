//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"ego/services/learning/config"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"
	"ego/services/learning/internal/service"
)

func TestRoomLifecycleWorkerAndReconcile(t *testing.T) {
	db := learningTestDB(t)

	ctx, now := context.Background(), time.Now()
	classroom := &model.Classroom{Code: "ROOM002", Name: "Lifecycle class", TeacherID: "teacher", Active: true}
	if err := db.Create(classroom).Error; err != nil {
		t.Fatal(err)
	}
	approved, joinedAt := true, now.Add(-time.Hour)
	if err := db.Create(&model.Member{ClassroomID: classroom.ID, UserID: "student", Approved: &approved, JoinedAt: &joinedAt}).Error; err != nil {
		t.Fatal(err)
	}

	repo := repository.NewRepository(db)
	mediaClient := &fakeRoomMedia{created: map[string]bool{}, participants: map[string][]string{}, listFailures: 2}
	svc := service.New(repo, &config.AppConfig{}, nil, testIdentityClient{}, nil, mediaClient, nil, nil)

	liveSchedule := &model.Schedule{ClassroomID: classroom.ID, Title: "Live", StartsAt: now.Add(-time.Hour), EndsAt: now.Add(time.Hour)}
	if err := db.Create(liveSchedule).Error; err != nil {
		t.Fatal(err)
	}
	startedAt := now.Add(-time.Hour)
	liveRoom := &model.Room{ScheduleID: liveSchedule.ID, Status: model.RoomStatusLive, StartedAt: &startedAt}
	if err := db.Create(liveRoom).Error; err != nil {
		t.Fatal(err)
	}
	liveName := fmt.Sprintf("classroom-room-%d", liveRoom.ID)
	mediaClient.participants[liveName] = []string{"teacher", "student"}
	if _, err := svc.ProcessRoomLifecycle(ctx); err != nil {
		t.Fatal(err)
	}
	if mediaClient.listCalls != 3 {
		t.Fatalf("expected reconcile to retry twice, calls=%d", mediaClient.listCalls)
	}
	var openCount int64
	if err := db.Model(&model.RoomParticipant{}).Where("room_id = ? AND left_at IS NULL", liveRoom.ID).Count(&openCount).Error; err != nil || openCount != 2 {
		t.Fatalf("expected reconcile to open host and student sessions, count=%d err=%v", openCount, err)
	}
	mediaClient.participants[liveName] = []string{"teacher"}
	if _, err := svc.ProcessRoomLifecycle(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.RoomParticipant{}).Where("room_id = ? AND user_id = ? AND left_at IS NULL", liveRoom.ID, "student").Count(&openCount).Error; err != nil || openCount != 0 {
		t.Fatalf("expected reconcile to close missing student session, count=%d err=%v", openCount, err)
	}

	// LiveKit đã dọn phòng trống: không retry như lỗi, coi là không ai kết nối.
	mediaClient.closedRooms = map[string]bool{liveName: true}
	listCallsBefore := mediaClient.listCalls
	if _, err := svc.ProcessRoomLifecycle(ctx); err != nil {
		t.Fatal(err)
	}
	if mediaClient.listCalls != listCallsBefore+1 {
		t.Fatalf("expected closed media room not to be retried, calls=%d", mediaClient.listCalls-listCallsBefore)
	}
	if err := db.Model(&model.RoomParticipant{}).Where("room_id = ? AND left_at IS NULL", liveRoom.ID).Count(&openCount).Error; err != nil || openCount != 0 {
		t.Fatalf("expected closed media room to close all open sessions, count=%d err=%v", openCount, err)
	}
	if room, err := repo.GetRoomByID(ctx, liveRoom.ID); err != nil || room.Status != model.RoomStatusLive || room.HostAbsentSince == nil {
		t.Fatalf("expected room to stay live with host marked absent: room=%#v err=%v", room, err)
	}

	expiredSchedule := &model.Schedule{ClassroomID: classroom.ID, Title: "Never started", StartsAt: now.Add(-2 * time.Hour), EndsAt: now.Add(-time.Hour)}
	if err := db.Create(expiredSchedule).Error; err != nil {
		t.Fatal(err)
	}
	expiredRoom := &model.Room{ScheduleID: expiredSchedule.ID, Status: model.RoomStatusScheduled}
	if err := db.Create(expiredRoom).Error; err != nil {
		t.Fatal(err)
	}

	absentSchedule := &model.Schedule{ClassroomID: classroom.ID, Title: "Host absent", StartsAt: now.Add(-2 * time.Hour), EndsAt: now.Add(-20 * time.Minute)}
	if err := db.Create(absentSchedule).Error; err != nil {
		t.Fatal(err)
	}
	absentSince := now.Add(-11 * time.Minute)
	absentRoom := &model.Room{ScheduleID: absentSchedule.ID, Status: model.RoomStatusLive, StartedAt: &startedAt, HostAbsentSince: &absentSince}
	if err := db.Create(absentRoom).Error; err != nil {
		t.Fatal(err)
	}

	limitSchedule := &model.Schedule{ClassroomID: classroom.ID, Title: "Absolute limit", StartsAt: now.Add(-5 * time.Hour), EndsAt: now.Add(-4 * time.Hour)}
	if err := db.Create(limitSchedule).Error; err != nil {
		t.Fatal(err)
	}
	limitRoom := &model.Room{ScheduleID: limitSchedule.ID, Status: model.RoomStatusLive, StartedAt: &startedAt}
	if err := db.Create(limitRoom).Error; err != nil {
		t.Fatal(err)
	}

	processed, err := svc.ProcessRoomLifecycle(ctx)
	if err != nil || processed != 3 {
		t.Fatalf("expected three rooms to end, processed=%d err=%v", processed, err)
	}
	for _, roomID := range []uint{expiredRoom.ID, absentRoom.ID, limitRoom.ID} {
		room, err := repo.GetRoomByID(ctx, roomID)
		if err != nil || room.Status != model.RoomStatusEnded || room.EndedAt == nil {
			t.Fatalf("room %d was not ended: room=%#v err=%v", roomID, room, err)
		}
	}
}

func TestRoomReconcileKeepsConcurrentEnd(t *testing.T) {
	db := learningTestDB(t)
	ctx, now := context.Background(), time.Now()
	classroom := &model.Classroom{Code: "ROOM003", Name: "Reconcile end", TeacherID: "teacher", Active: true}
	if err := db.Create(classroom).Error; err != nil {
		t.Fatal(err)
	}
	repo := repository.NewRepository(db)
	mediaClient := &fakeRoomMedia{created: map[string]bool{}, participants: map[string][]string{}}
	svc := service.New(repo, &config.AppConfig{}, nil, testIdentityClient{}, nil, mediaClient, nil, nil)

	schedule := &model.Schedule{ClassroomID: classroom.ID, Title: "Live", StartsAt: now.Add(-time.Hour), EndsAt: now.Add(time.Hour)}
	if err := db.Create(schedule).Error; err != nil {
		t.Fatal(err)
	}
	startedAt := now.Add(-time.Hour)
	room := &model.Room{ScheduleID: schedule.ID, Status: model.RoomStatusLive, StartedAt: &startedAt}
	if err := db.Create(room).Error; err != nil {
		t.Fatal(err)
	}
	mediaClient.participants[fmt.Sprintf("classroom-room-%d", room.ID)] = []string{"student"}
	mediaClient.onList = func() {
		mediaClient.onList = nil
		if _, err := svc.EndRoom(ctx, "teacher", room.ID); err != nil {
			t.Errorf("end room: %v", err)
		}
	}
	if _, err := svc.ProcessRoomLifecycle(ctx); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetRoomByID(ctx, room.ID)
	if err != nil || got.Status != model.RoomStatusEnded || got.EndedAt == nil {
		t.Fatalf("room must stay ended: room=%#v err=%v", got, err)
	}
}

func TestRoomReconcileKeepsWebhookHostAbsence(t *testing.T) {
	db := learningTestDB(t)
	ctx, now := context.Background(), time.Now()
	classroom := &model.Classroom{Code: "ROOM004", Name: "Reconcile absence", TeacherID: "teacher", Active: true}
	if err := db.Create(classroom).Error; err != nil {
		t.Fatal(err)
	}
	repo := repository.NewRepository(db)
	mediaClient := &fakeRoomMedia{created: map[string]bool{}, participants: map[string][]string{}}
	svc := service.New(repo, &config.AppConfig{}, nil, testIdentityClient{}, nil, mediaClient, nil, nil)

	schedule := &model.Schedule{ClassroomID: classroom.ID, Title: "Live", StartsAt: now.Add(-time.Hour), EndsAt: now.Add(time.Hour)}
	if err := db.Create(schedule).Error; err != nil {
		t.Fatal(err)
	}
	startedAt := now.Add(-time.Hour)
	room := &model.Room{ScheduleID: schedule.ID, Status: model.RoomStatusLive, StartedAt: &startedAt}
	if err := db.Create(room).Error; err != nil {
		t.Fatal(err)
	}
	roomName := fmt.Sprintf("classroom-room-%d", room.ID)
	if err := svc.HandleParticipantJoined(ctx, roomName, "teacher", startedAt); err != nil {
		t.Fatal(err)
	}
	leftAt := now.Add(-time.Minute).Truncate(time.Microsecond)
	mediaClient.onList = func() {
		mediaClient.onList = nil
		if err := svc.HandleParticipantLeft(ctx, roomName, "teacher", leftAt); err != nil {
			t.Errorf("participant left: %v", err)
		}
	}
	if _, err := svc.ProcessRoomLifecycle(ctx); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetRoomByID(ctx, room.ID)
	if err != nil || got.HostAbsentSince == nil || !got.HostAbsentSince.Equal(leftAt) {
		t.Fatalf("host absence must keep the webhook time %v: room=%#v err=%v", leftAt, got, err)
	}

	// Danh sách LiveKit vẫn còn giáo viên nhưng webhook left đến ngay sau khi lấy danh sách:
	// worker chỉ xóa thời điểm vắng mặt ghi trước lúc lấy danh sách.
	if err := svc.HandleParticipantJoined(ctx, roomName, "teacher", now); err != nil {
		t.Fatal(err)
	}
	mediaClient.participants[roomName] = []string{"teacher"}
	var webhookLeftAt time.Time
	mediaClient.onList = func() {
		mediaClient.onList = nil
		webhookLeftAt = time.Now().Truncate(time.Microsecond)
		if err := svc.HandleParticipantLeft(ctx, roomName, "teacher", webhookLeftAt); err != nil {
			t.Errorf("participant left: %v", err)
		}
	}
	if _, err := svc.ProcessRoomLifecycle(ctx); err != nil {
		t.Fatal(err)
	}
	got, err = repo.GetRoomByID(ctx, room.ID)
	if err != nil || got.HostAbsentSince == nil || !got.HostAbsentSince.Equal(webhookLeftAt) {
		t.Fatalf("absence written after the snapshot must survive: want %v room=%#v err=%v", webhookLeftAt, got, err)
	}
}
