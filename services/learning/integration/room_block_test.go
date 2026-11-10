//go:build integration

package integration_test

import (
	"context"
	"testing"
	"time"

	"ego/services/learning/config"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/media"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"
	"ego/services/learning/internal/service"
)

// TestRemoveUserFromLiveRooms: khoá học sinh thì ngắt khỏi phòng LIVE, khoá giáo viên thì kết thúc phòng.
func TestRemoveUserFromLiveRooms(t *testing.T) {
	db := learningTestDB(t)
	ctx := context.Background()
	classroom := &model.Classroom{Code: "BLOCK01", Name: "Block class", TeacherID: "teacher", Active: true}
	if err := db.Create(classroom).Error; err != nil {
		t.Fatal(err)
	}
	approved := true
	joinedAt := time.Now()
	if err := db.Create(&model.Member{ClassroomID: classroom.ID, UserID: "student", Approved: &approved, JoinedAt: &joinedAt}).Error; err != nil {
		t.Fatal(err)
	}

	mediaClient := &fakeRoomMedia{created: map[string]bool{}, participants: map[string][]string{}}
	svc := service.New(repository.NewRepository(db), &config.AppConfig{LiveKitPublicURL: "ws://localhost:7880"}, nil, testIdentityClient{}, nil, mediaClient, nil, nil)
	now := time.Now()
	schedule, err := svc.CreateSchedule(ctx, "teacher", classroom.ID, dto.CreateScheduleRequest{Title: "Live", StartsAt: now.Add(5 * time.Minute), EndsAt: now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	roomID := schedule.Room.ID
	if _, err := svc.StartRoom(ctx, "teacher", roomID); err != nil {
		t.Fatal(err)
	}

	removed, ended, removeErr := svc.RemoveUserFromLiveRooms(ctx, "student")
	if removeErr != nil || removed != 1 || ended != 0 || len(mediaClient.removed) != 1 || mediaClient.removed[0] != media.RoomName(roomID)+":student" {
		t.Fatalf("student: removed=%d ended=%d media=%v err=%v", removed, ended, mediaClient.removed, removeErr)
	}
	if removed, ended, err := svc.RemoveUserFromLiveRooms(ctx, "outsider"); err != nil || removed != 0 || ended != 0 {
		t.Fatalf("outsider: removed=%d ended=%d err=%v", removed, ended, err)
	}

	removed, ended, removeErr = svc.RemoveUserFromLiveRooms(ctx, "teacher")
	if removeErr != nil || removed != 0 || ended != 1 || mediaClient.deleteCalls != 1 {
		t.Fatalf("teacher: removed=%d ended=%d deleteCalls=%d err=%v", removed, ended, mediaClient.deleteCalls, removeErr)
	}
	var room model.Room
	if err := db.First(&room, roomID).Error; err != nil || room.Status != model.RoomStatusEnded {
		t.Fatalf("room status=%s err=%v, want ENDED", room.Status, err)
	}
	if _, _, err := svc.RemoveUserFromLiveRooms(ctx, " "); err == nil {
		t.Fatal("expected empty user id to be rejected")
	}
}

// TestBlockedUserKickedOnJoin: user bị khoá vào lại phòng bằng token LiveKit cũ thì webhook join đá ra
// ngay và không mở phiên điểm danh; user thường vẫn được mở phiên.
func TestBlockedUserKickedOnJoin(t *testing.T) {
	db := learningTestDB(t)
	ctx := context.Background()
	classroom := &model.Classroom{Code: "BLOCK02", Name: "Block class", TeacherID: "teacher", Active: true}
	if err := db.Create(classroom).Error; err != nil {
		t.Fatal(err)
	}
	mediaClient := &fakeRoomMedia{created: map[string]bool{}, participants: map[string][]string{}}
	svc := service.New(repository.NewRepository(db), &config.AppConfig{LiveKitPublicURL: "ws://localhost:7880"}, nil, testIdentityClient{}, nil, mediaClient, nil, nil)
	now := time.Now()
	schedule, err := svc.CreateSchedule(ctx, "teacher", classroom.ID, dto.CreateScheduleRequest{Title: "Live", StartsAt: now.Add(5 * time.Minute), EndsAt: now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	roomID := schedule.Room.ID
	if _, err := svc.StartRoom(ctx, "teacher", roomID); err != nil {
		t.Fatal(err)
	}
	roomName := media.RoomName(roomID)

	if err := svc.HandleParticipantJoined(ctx, roomName, "blocked-student", now); err != nil {
		t.Fatal(err)
	}
	if len(mediaClient.removed) != 1 || mediaClient.removed[0] != roomName+":blocked-student" {
		t.Fatalf("media removed=%v, want [%s:blocked-student]", mediaClient.removed, roomName)
	}
	var sessions int64
	db.Model(&model.RoomParticipant{}).Where("room_id = ? AND user_id = ?", roomID, "blocked-student").Count(&sessions)
	if sessions != 0 {
		t.Fatalf("blocked user có %d phiên điểm danh, want 0", sessions)
	}

	if err := svc.HandleParticipantJoined(ctx, roomName, "teacher", now); err != nil {
		t.Fatal(err)
	}
	db.Model(&model.RoomParticipant{}).Where("room_id = ? AND user_id = ?", roomID, "teacher").Count(&sessions)
	if sessions != 1 || len(mediaClient.removed) != 1 {
		t.Fatalf("teacher sessions=%d removed=%v, want 1 phiên và không bị đá", sessions, mediaClient.removed)
	}
}
