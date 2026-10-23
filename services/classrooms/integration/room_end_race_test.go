//go:build integration

package integration_test

import (
	"context"
	"testing"
	"time"

	"ego/services/classrooms/config"
	"ego/services/classrooms/internal/dto"
	"ego/services/classrooms/internal/media"
	"ego/services/classrooms/internal/model"
	"ego/services/classrooms/internal/repository"
	"ego/services/classrooms/internal/service"
)

// LiveKit gửi participant_left/joined trong lúc EndRoom đang chạy. Webhook đến
// muộn không được đưa room đã kết thúc trở lại LIVE.
func TestLateHostWebhooksDoNotReopenEndedRoom(t *testing.T) {
	db := classroomTestDB(t)
	ctx := context.Background()
	now := time.Now()
	classroom := &model.Classroom{Code: "RACE001", Name: "Race", TeacherID: "teacher", Active: true}
	if err := db.Create(classroom).Error; err != nil {
		t.Fatal(err)
	}
	repo := repository.NewRepository(db)
	users, exams := classroomTestClients(t)
	svc := service.New(repo, &config.AppConfig{}, users, exams)
	svc.SetMediaClient(&fakeRoomMedia{created: map[string]bool{}, participants: map[string][]string{}})
	created, err := svc.CreateSchedule(ctx, "teacher", classroom.ID, dto.CreateScheduleRequest{Title: "Race", StartsAt: now.Add(-time.Minute), EndsAt: now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	roomID := created.Room.ID
	roomName := media.RoomName(roomID)
	if _, err := svc.StartRoom(ctx, "teacher", roomID); err != nil {
		t.Fatal(err)
	}
	if err := svc.HandleParticipantJoined(ctx, roomName, "teacher", now); err != nil {
		t.Fatal(err)
	}

	// Bản ghi nạp trước khi room kết thúc, như webhook đang xử lý dở.
	stale, err := repo.GetRoomByID(ctx, roomID)
	if err != nil || stale.Status != model.RoomStatusLive {
		t.Fatalf("stale room = %#v err=%v", stale, err)
	}
	if _, err := svc.EndRoom(ctx, "teacher", roomID); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetRoomHostAbsentSince(ctx, stale.ID, &now); err != nil {
		t.Fatal(err)
	}
	if err := svc.HandleParticipantLeft(ctx, roomName, "teacher", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := svc.HandleParticipantJoined(ctx, roomName, "teacher", time.Now()); err != nil {
		t.Fatal(err)
	}

	ended, err := repo.GetRoomByID(ctx, roomID)
	if err != nil {
		t.Fatal(err)
	}
	if ended.Status != model.RoomStatusEnded || ended.EndedAt == nil || ended.HostAbsentSince != nil {
		t.Fatalf("late host webhooks reopened the room: status=%s endedAt=%v hostAbsentSince=%v", ended.Status, ended.EndedAt, ended.HostAbsentSince)
	}
}
