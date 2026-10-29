//go:build integration

package integration_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"ego/services/learning/config"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/media"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"
	"ego/services/learning/internal/service"
	learningSocket "ego/services/learning/internal/socket"
)

func TestRaiseAndLowerHands(t *testing.T) {
	db := learningTestDB(t)
	ctx := context.Background()
	now := time.Now()
	classroom := &model.Classroom{Code: "HANDS01", Name: "Hands", TeacherID: "teacher", Active: true}
	if err := db.Create(classroom).Error; err != nil {
		t.Fatal(err)
	}
	approved, joinedAt := true, now
	for _, student := range []string{"an", "binh", "chi"} {
		if err := db.Create(&model.Member{ClassroomID: classroom.ID, UserID: student, Approved: &approved, JoinedAt: &joinedAt}).Error; err != nil {
			t.Fatal(err)
		}
	}
	repo := repository.NewRepository(db)
	var events []sentRoomEvent
	publisher := func(recipient string, payload any) {
		if event, ok := payload.(learningSocket.RoomEvent); ok {
			activity, _ := event.Payload.(learningSocket.ParticipantActivityPayload)
			events = append(events, sentRoomEvent{recipient: recipient, eventType: event.Type, userID: activity.UserID})
		}
	}
	svc := service.New(repo, &config.AppConfig{}, nil, testIdentityClient{}, nil, &fakeRoomMedia{created: map[string]bool{}, participants: map[string][]string{}}, publisher, nil)
	takeEvents := func() []sentRoomEvent {
		taken := events
		events = nil
		return taken
	}

	created, err := svc.CreateSchedule(ctx, "teacher", classroom.ID, dto.CreateScheduleRequest{Title: "Hands", StartsAt: now.Add(-time.Minute), EndsAt: now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	roomID := created.Room.ID
	if _, err := svc.SetHandRaised(ctx, "an", classroom.ID, roomID, true); !errors.Is(err, service.ErrRoomNotLive) {
		t.Fatalf("hand before the room starts = %v, want not live", err)
	}
	if _, err := svc.StartRoom(ctx, "teacher", roomID); err != nil {
		t.Fatal(err)
	}
	takeEvents()

	if _, err := svc.SetHandRaised(ctx, "teacher", classroom.ID, roomID, true); !errors.Is(err, service.ErrHandTeacher) {
		t.Fatalf("teacher hand = %v, want ErrHandTeacher", err)
	}
	if _, err := svc.SetHandRaised(ctx, "outsider", classroom.ID, roomID, true); !errors.Is(err, service.ErrClassroomForbidden) {
		t.Fatalf("outsider hand = %v, want forbidden", err)
	}

	first, err := svc.SetHandRaised(ctx, "binh", classroom.ID, roomID, true)
	if err != nil || first.HandRaisedAt == nil {
		t.Fatalf("raise = %#v err=%v", first, err)
	}
	if got := takeEvents(); !slices.Equal(got, []sentRoomEvent{
		{recipient: "teacher", eventType: learningSocket.EventHandRaised, userID: "binh"},
		{recipient: "binh", eventType: learningSocket.EventHandRaised, userID: "binh"},
	}) {
		t.Fatalf("raise events = %#v", got)
	}
	// Giơ tay lặp lại (ví dụ sau refresh) giữ nguyên thứ tự ban đầu.
	again, err := svc.SetHandRaised(ctx, "binh", classroom.ID, roomID, true)
	if err != nil || again.HandRaisedAt.Sub(*first.HandRaisedAt).Abs() > time.Millisecond || len(takeEvents()) != 0 {
		t.Fatalf("repeated raise must keep the original time: %#v err=%v", again, err)
	}
	for _, student := range []string{"an", "chi"} {
		time.Sleep(5 * time.Millisecond)
		if _, err := svc.SetHandRaised(ctx, student, classroom.ID, roomID, true); err != nil {
			t.Fatal(err)
		}
	}
	takeEvents()

	// Snapshot vẫn còn sau khi "refresh" (rời và vào lại phòng).
	if err := svc.HandleParticipantLeft(ctx, media.RoomName(roomID), "an", time.Now()); err != nil {
		t.Fatal(err)
	}
	activity, err := svc.GetRoomActivity(ctx, "an", classroom.ID, roomID)
	if err != nil || len(activity.Participants) != 1 || activity.Participants[0].HandRaisedAt == nil {
		t.Fatalf("hand must survive leaving the room: %#v err=%v", activity, err)
	}

	if _, err := svc.LowerHands(ctx, "an", classroom.ID, roomID, nil); !errors.Is(err, service.ErrClassroomForbidden) {
		t.Fatalf("student lowering all hands = %v, want forbidden", err)
	}
	outsider := "outsider"
	if _, err := svc.LowerHands(ctx, "teacher", classroom.ID, roomID, &outsider); !errors.Is(err, service.ErrRoomParticipantInvalid) {
		t.Fatalf("lower outsider = %v, want invalid participant", err)
	}
	chi := "chi"
	one, err := svc.LowerHands(ctx, "teacher", classroom.ID, roomID, &chi)
	if err != nil || !slices.Equal(one.UserIDs, []string{"chi"}) {
		t.Fatalf("lower one = %#v err=%v", one, err)
	}
	if got := takeEvents(); !slices.Contains(got, sentRoomEvent{recipient: "chi", eventType: learningSocket.EventHandLowered, userID: "chi"}) {
		t.Fatalf("lower one events = %#v", got)
	}

	all, err := svc.LowerHands(ctx, "teacher", classroom.ID, roomID, nil)
	if err != nil || !slices.Equal(all.UserIDs, []string{"binh", "an"}) {
		t.Fatalf("lower all must follow raise order: %#v err=%v", all, err)
	}
	if got := takeEvents(); len(got) != 4 {
		t.Fatalf("lower all must notify the teacher and each student: %#v", got)
	}
	if none, err := svc.LowerHands(ctx, "teacher", classroom.ID, roomID, nil); err != nil || none.UserIDs == nil || len(none.UserIDs) != 0 {
		t.Fatalf("lowering with no hands = %#v err=%v", none, err)
	}
	hostActivity, err := svc.GetRoomActivity(ctx, "teacher", classroom.ID, roomID)
	if err != nil {
		t.Fatal(err)
	}
	for _, state := range hostActivity.Participants {
		if state.HandRaisedAt != nil {
			t.Fatalf("hand still raised after lowering all: %#v", state)
		}
	}

	if _, err := svc.SetHandRaised(ctx, "an", classroom.ID, roomID, true); err != nil {
		t.Fatal(err)
	}
	if lowered, err := svc.SetHandRaised(ctx, "an", classroom.ID, roomID, false); err != nil || lowered.HandRaisedAt != nil {
		t.Fatalf("self lower = %#v err=%v", lowered, err)
	}
}
