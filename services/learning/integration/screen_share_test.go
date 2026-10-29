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

type sentRoomEvent struct {
	recipient string
	eventType string
	userID    string
}

func TestScreenShareRequestApprovalAndReplacement(t *testing.T) {
	db := learningTestDB(t)
	ctx := context.Background()
	now := time.Now()
	classroom := &model.Classroom{Code: "SHARE01", Name: "Screen share", TeacherID: "teacher", Active: true}
	if err := db.Create(classroom).Error; err != nil {
		t.Fatal(err)
	}
	approved, joinedAt := true, now
	for _, student := range []string{"student", "student-2"} {
		if err := db.Create(&model.Member{ClassroomID: classroom.ID, UserID: student, Approved: &approved, JoinedAt: &joinedAt}).Error; err != nil {
			t.Fatal(err)
		}
	}

	repo := repository.NewRepository(db)
	mediaClient := &fakeRoomMedia{created: map[string]bool{}, participants: map[string][]string{}, sharers: map[string][]string{}, disconnected: map[string]bool{}}
	var events []sentRoomEvent
	publisher := func(recipient string, payload any) {
		if event, ok := payload.(learningSocket.RoomEvent); ok {
			activity, _ := event.Payload.(learningSocket.ParticipantActivityPayload)
			events = append(events, sentRoomEvent{recipient: recipient, eventType: event.Type, userID: activity.UserID})
		}
	}
	svc := service.New(repo, &config.AppConfig{}, nil, testIdentityClient{}, nil, mediaClient, publisher, nil)
	takeEvents := func() []sentRoomEvent {
		taken := events
		events = nil
		return taken
	}

	created, err := svc.CreateSchedule(ctx, "teacher", classroom.ID, dto.CreateScheduleRequest{Title: "Share", StartsAt: now.Add(-time.Minute), EndsAt: now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	roomID := created.Room.ID
	roomName := media.RoomName(roomID)
	if err := svc.StartScreenShare(ctx, "teacher", classroom.ID, roomID); !errors.Is(err, service.ErrRoomNotLive) {
		t.Fatalf("screen share before the room starts = %v, want not live", err)
	}
	if _, err := svc.StartRoom(ctx, "teacher", roomID); err != nil {
		t.Fatal(err)
	}
	takeEvents()

	// Giáo viên chia sẻ trực tiếp; học sinh phải xin và được duyệt trước.
	if _, err := svc.RequestScreenShare(ctx, "teacher", classroom.ID, roomID); !errors.Is(err, service.ErrScreenShareTeacherDirect) {
		t.Fatalf("teacher request = %v, want direct share error", err)
	}
	if err := svc.StartScreenShare(ctx, "student", classroom.ID, roomID); !errors.Is(err, service.ErrScreenShareNotApproved) {
		t.Fatalf("unapproved student start = %v, want not approved", err)
	}
	if _, err := svc.RequestScreenShare(ctx, "outsider", classroom.ID, roomID); !errors.Is(err, service.ErrClassroomForbidden) {
		t.Fatalf("outsider request = %v, want forbidden", err)
	}
	mediaClient.sharers[roomName] = []string{"teacher"}
	if err := svc.StartScreenShare(ctx, "teacher", classroom.ID, roomID); err != nil || len(mediaClient.stopped) != 0 {
		t.Fatalf("teacher restart must not stop itself: stopped=%v err=%v", mediaClient.stopped, err)
	}

	request, err := svc.RequestScreenShare(ctx, "student", classroom.ID, roomID)
	if err != nil || request.ScreenShareRequestedAt == nil || request.ScreenShareApprovedAt != nil {
		t.Fatalf("request = %#v, err=%v", request, err)
	}
	if got := takeEvents(); !slices.Equal(got, []sentRoomEvent{
		{recipient: "teacher", eventType: learningSocket.EventScreenShareRequested, userID: "student"},
		{recipient: "student", eventType: learningSocket.EventScreenShareRequested, userID: "student"},
	}) {
		t.Fatalf("request events = %#v", got)
	}
	if again, err := svc.RequestScreenShare(ctx, "student", classroom.ID, roomID); err != nil || again.ScreenShareRequestedAt.Sub(*request.ScreenShareRequestedAt).Abs() > time.Millisecond || len(takeEvents()) != 0 {
		t.Fatalf("repeated request must be idempotent: %#v err=%v", again, err)
	}

	if _, err := svc.ApproveScreenShare(ctx, "student-2", classroom.ID, roomID, "student"); !errors.Is(err, service.ErrClassroomForbidden) {
		t.Fatalf("student approving = %v, want forbidden", err)
	}
	if _, err := svc.ApproveScreenShare(ctx, "teacher", classroom.ID, roomID, "student-2"); !errors.Is(err, service.ErrScreenShareNotRequested) {
		t.Fatalf("approve without request = %v, want not requested", err)
	}
	if _, err := svc.ApproveScreenShare(ctx, "teacher", classroom.ID, roomID, "outsider"); !errors.Is(err, service.ErrRoomParticipantInvalid) {
		t.Fatalf("approve outsider = %v, want invalid participant", err)
	}
	mediaClient.disconnected["student"] = true
	if _, err := svc.ApproveScreenShare(ctx, "teacher", classroom.ID, roomID, "student"); !errors.Is(err, service.ErrParticipantNotConnected) {
		t.Fatalf("approve disconnected = %v, want not connected", err)
	}
	delete(mediaClient.disconnected, "student")
	approval, err := svc.ApproveScreenShare(ctx, "teacher", classroom.ID, roomID, "student")
	if err != nil || approval.ScreenShareApprovedAt == nil || !mediaClient.screenSharePermission["student"] {
		t.Fatalf("approve = %#v permission=%v err=%v", approval, mediaClient.screenSharePermission, err)
	}
	takeEvents()

	// Lượt chia sẻ hợp lệ mới dừng lượt của giáo viên trước khi bắt đầu.
	if err := svc.StartScreenShare(ctx, "student", classroom.ID, roomID); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(mediaClient.stopped, []string{"teacher"}) || len(mediaClient.sharers[roomName]) != 0 {
		t.Fatalf("teacher share must be replaced: stopped=%v sharers=%v", mediaClient.stopped, mediaClient.sharers[roomName])
	}
	if _, revoked := mediaClient.screenSharePermission["teacher"]; revoked {
		t.Fatal("the teacher's screen share permission must never be revoked")
	}
	if got := takeEvents(); !slices.Equal(got, []sentRoomEvent{{recipient: "teacher", eventType: learningSocket.EventScreenShareStopped, userID: "teacher"}}) {
		t.Fatalf("replacement events = %#v", got)
	}
	mediaClient.sharers[roomName] = []string{"student"}
	if err := svc.HandleScreenSharePublished(ctx, roomName, "student"); err != nil || len(mediaClient.stopped) != 1 {
		t.Fatalf("approved publish must be kept: stopped=%v err=%v", mediaClient.stopped, err)
	}

	// Track publish không qua API của học sinh chưa được duyệt bị dừng ngay.
	mediaClient.sharers[roomName] = []string{"student", "student-2"}
	if err := svc.HandleScreenSharePublished(ctx, roomName, "student-2"); err != nil {
		t.Fatal(err)
	}
	if mediaClient.stopped[len(mediaClient.stopped)-1] != "student-2" || mediaClient.screenSharePermission["student-2"] {
		t.Fatalf("unapproved publish must be stopped: stopped=%v permission=%v", mediaClient.stopped, mediaClient.screenSharePermission)
	}
	takeEvents()

	if err := svc.StopScreenShare(ctx, "student-2", classroom.ID, roomID, "student"); !errors.Is(err, service.ErrClassroomForbidden) {
		t.Fatalf("student stopping another student = %v, want forbidden", err)
	}
	if err := svc.StopScreenShare(ctx, "teacher", classroom.ID, roomID, "student"); err != nil {
		t.Fatal(err)
	}
	if mediaClient.screenSharePermission["student"] {
		t.Fatal("stopping the share must revoke the student's permission")
	}
	if got := takeEvents(); !slices.Contains(got, sentRoomEvent{recipient: "student", eventType: learningSocket.EventScreenShareStopped, userID: "student"}) {
		t.Fatalf("stop events = %#v", got)
	}
	state, repoErr := repo.FindRoomParticipantState(ctx, roomID, "student")
	if repoErr != nil || state.ScreenShareRequestedAt != nil || state.ScreenShareApprovedAt != nil {
		t.Fatalf("stopped share must clear request and approval: %#v err=%v", state, repoErr)
	}
	if err := svc.StartScreenShare(ctx, "student", classroom.ID, roomID); !errors.Is(err, service.ErrScreenShareNotApproved) {
		t.Fatalf("next share must need a new request, got %v", err)
	}

	// Từ chối xóa yêu cầu; nếu đã duyệt thì thu hồi quyền.
	if _, err := svc.RequestScreenShare(ctx, "student", classroom.ID, roomID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ApproveScreenShare(ctx, "teacher", classroom.ID, roomID, "student"); err != nil {
		t.Fatal(err)
	}
	takeEvents()
	rejected, err := svc.RejectScreenShare(ctx, "teacher", classroom.ID, roomID, "student")
	if err != nil || rejected.ScreenShareRequestedAt != nil || mediaClient.screenSharePermission["student"] {
		t.Fatalf("reject = %#v permission=%v err=%v", rejected, mediaClient.screenSharePermission, err)
	}
	if got := takeEvents(); !slices.Contains(got, sentRoomEvent{recipient: "student", eventType: learningSocket.EventScreenShareRejected, userID: "student"}) {
		t.Fatalf("reject events = %#v", got)
	}
	if _, err := svc.RejectScreenShare(ctx, "teacher", classroom.ID, roomID, "student"); !errors.Is(err, service.ErrScreenShareNotRequested) {
		t.Fatalf("second reject = %v, want not requested", err)
	}

	// Học sinh tự dừng (track unpublished) hoặc rời phòng cũng xóa phê duyệt.
	for _, end := range []func() error{
		func() error { return svc.HandleScreenShareUnpublished(ctx, roomName, "student") },
		func() error { return svc.HandleParticipantLeft(ctx, roomName, "student", time.Now()) },
	} {
		if _, err := svc.RequestScreenShare(ctx, "student", classroom.ID, roomID); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.ApproveScreenShare(ctx, "teacher", classroom.ID, roomID, "student"); err != nil {
			t.Fatal(err)
		}
		takeEvents()
		if err := end(); err != nil {
			t.Fatal(err)
		}
		state, err := repo.FindRoomParticipantState(ctx, roomID, "student")
		if err != nil || state.ScreenShareApprovedAt != nil || state.ScreenShareRequestedAt != nil {
			t.Fatalf("ended share must clear approval: %#v err=%v", state, err)
		}
		if got := takeEvents(); !slices.Contains(got, sentRoomEvent{recipient: "teacher", eventType: learningSocket.EventScreenShareStopped, userID: "student"}) {
			t.Fatalf("end events = %#v", got)
		}
	}
	if err := svc.HandleScreenShareUnpublished(ctx, roomName, "student"); err != nil || len(takeEvents()) != 0 {
		t.Fatalf("duplicate unpublish webhook must be a no-op: err=%v", err)
	}

	if _, err := svc.EndRoom(ctx, "teacher", roomID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RequestScreenShare(ctx, "student", classroom.ID, roomID); !errors.Is(err, service.ErrRoomEnded) {
		t.Fatalf("request after end = %v, want ended", err)
	}
}
