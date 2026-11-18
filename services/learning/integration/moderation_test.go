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

func TestParticipantModerationAndAudit(t *testing.T) {
	db := learningTestDB(t)
	ctx := context.Background()
	now := time.Now()
	classroom := &model.Classroom{Code: "MODER01", Name: "Moderation", TeacherID: "teacher", Active: true}
	if err := db.Create(classroom).Error; err != nil {
		t.Fatal(err)
	}
	approved, joinedAt := true, now
	for _, student := range []string{"an", "binh"} {
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
	audits := func() []*model.RoomModerationAudit {
		t.Helper()
		list, err := repo.GetRoomModerationAudits(ctx, 0)
		if err != nil {
			t.Fatal(err)
		}
		return list
	}

	created, err := svc.CreateSchedule(ctx, "teacher", classroom.ID, dto.CreateScheduleRequest{Title: "Moderation", StartsAt: now.Add(-time.Minute), EndsAt: now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	roomID := created.Room.ID
	audits = func() []*model.RoomModerationAudit {
		t.Helper()
		list, err := repo.GetRoomModerationAudits(ctx, roomID)
		if err != nil {
			t.Fatal(err)
		}
		return list
	}
	if _, err := svc.StartRoom(ctx, "teacher", roomID); err != nil {
		t.Fatal(err)
	}
	takeEvents()

	// Chỉ giáo viên được thao tác và chỉ lên học sinh của lớp.
	if _, err := svc.MuteParticipant(ctx, "binh", classroom.ID, roomID, "an"); !errors.Is(err, service.ErrClassroomForbidden) {
		t.Fatalf("student muting = %v, want forbidden", err)
	}
	if _, err := svc.MuteParticipant(ctx, "teacher", classroom.ID, roomID, "teacher"); !errors.Is(err, service.ErrRoomParticipantInvalid) {
		t.Fatalf("muting the teacher = %v, want invalid participant", err)
	}
	if _, err := svc.MuteParticipant(ctx, "teacher", classroom.ID, roomID, "outsider"); !errors.Is(err, service.ErrRoomParticipantInvalid) {
		t.Fatalf("muting an outsider = %v, want invalid participant", err)
	}

	// Học sinh chưa kết nối vẫn bị tắt mic và token lần vào phòng sau tôn trọng điều đó.
	mediaClient.disconnected["an"] = true
	muted, err := svc.MuteParticipant(ctx, "teacher", classroom.ID, roomID, "an")
	if err != nil || muted.MicrophoneAllowed {
		t.Fatalf("mute = %#v err=%v", muted, err)
	}
	delete(mediaClient.disconnected, "an")
	if got := audits(); len(got) != 1 || got[0].Action != model.RoomModerationAuditActionMute || got[0].ActorUserID != "teacher" || got[0].TargetUserID != "an" {
		t.Fatalf("mute audit = %#v", got)
	}
	if got := takeEvents(); !slices.Contains(got, sentRoomEvent{recipient: "an", eventType: learningSocket.EventParticipantPermissionsUpdated, userID: "an"}) {
		t.Fatalf("mute events = %#v", got)
	}
	if _, err := svc.JoinRoom(ctx, "an", roomID); err != nil {
		t.Fatal(err)
	}
	if token := mediaClient.tokens["an"]; token.Microphone || !token.Camera || token.ScreenShare {
		t.Fatalf("muted student's token = %#v", token)
	}
	if _, err := svc.MuteParticipant(ctx, "teacher", classroom.ID, roomID, "an"); err != nil || len(audits()) != 1 || len(takeEvents()) != 0 {
		t.Fatalf("repeated mute must be a no-op: err=%v audits=%d", err, len(audits()))
	}

	// Duyệt chia sẻ màn hình không được trả lại mic đã bị tắt.
	if _, err := svc.RequestScreenShare(ctx, "an", classroom.ID, roomID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ApproveScreenShare(ctx, "teacher", classroom.ID, roomID, "an"); err != nil {
		t.Fatal(err)
	}
	if got := mediaClient.permissions["an"]; got != (media.PublishPermissions{Camera: true, ScreenShare: true}) {
		t.Fatalf("approval permissions = %#v", got)
	}

	// Giáo viên chỉ cấp lại quyền; học sinh vẫn tự bật thiết bị.
	allowed, err := svc.AllowParticipantMicrophone(ctx, "teacher", classroom.ID, roomID, "an")
	if err != nil || !allowed.MicrophoneAllowed {
		t.Fatalf("allow microphone = %#v err=%v", allowed, err)
	}
	if got := mediaClient.permissions["an"]; got != (media.PublishPermissions{Camera: true, Microphone: true, ScreenShare: true}) {
		t.Fatalf("allow must keep the screen share approval: %#v", got)
	}
	if got := takeEvents(); !slices.Contains(got, sentRoomEvent{recipient: "an", eventType: learningSocket.EventParticipantMicrophoneAllowed, userID: "an"}) {
		t.Fatalf("allow events = %#v", got)
	}
	if len(audits()) != 1 {
		t.Fatal("granting permission back is not a moderation audit entry")
	}

	disabled, err := svc.DisableParticipantCamera(ctx, "teacher", classroom.ID, roomID, "binh")
	if err != nil || disabled.CameraAllowed || !disabled.MicrophoneAllowed {
		t.Fatalf("disable camera = %#v err=%v", disabled, err)
	}
	if !slices.Contains(mediaClient.muted, "binh:0") || mediaClient.permissions["binh"] != (media.PublishPermissions{Microphone: true}) {
		t.Fatalf("camera must be muted and revoked: muted=%v permissions=%#v", mediaClient.muted, mediaClient.permissions["binh"])
	}
	if got := audits(); got[len(got)-1].Action != model.RoomModerationAuditActionDisableCamera {
		t.Fatalf("disable camera audit = %#v", got[len(got)-1])
	}
	if _, err := svc.AllowParticipantCamera(ctx, "teacher", classroom.ID, roomID, "binh"); err != nil || !mediaClient.permissions["binh"].Camera {
		t.Fatalf("allow camera err=%v permissions=%#v", err, mediaClient.permissions["binh"])
	}

	// Mute all bỏ qua giáo viên và học sinh đã bị tắt mic.
	if _, err := svc.MuteParticipant(ctx, "teacher", classroom.ID, roomID, "binh"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AllowParticipantMicrophone(ctx, "teacher", classroom.ID, roomID, "binh"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.JoinRoom(ctx, "teacher", roomID); err != nil {
		t.Fatal(err)
	}
	if token := mediaClient.tokens["teacher"]; token != media.HostPermissions() {
		t.Fatalf("teacher token = %#v", token)
	}
	auditsBefore := len(audits())
	all, err := svc.MuteAllParticipants(ctx, "teacher", classroom.ID, roomID)
	if err != nil || !slices.Equal(all.UserIDs, []string{"an", "binh"}) {
		t.Fatalf("mute all = %#v err=%v", all, err)
	}
	if len(audits()) != auditsBefore+2 {
		t.Fatalf("mute all must audit each muted student: %d -> %d", auditsBefore, len(audits()))
	}
	if again, err := svc.MuteAllParticipants(ctx, "teacher", classroom.ID, roomID); err != nil || len(again.UserIDs) != 0 {
		t.Fatalf("second mute all = %#v err=%v", again, err)
	}
	if _, err := svc.MuteAllParticipants(ctx, "an", classroom.ID, roomID); !errors.Is(err, service.ErrClassroomForbidden) {
		t.Fatalf("student mute all = %v, want forbidden", err)
	}
	takeEvents()

	// Xóa khỏi phòng: ngắt kết nối, khóa vào lại, giữ thành viên của lớp.
	if _, err := svc.SetHandRaised(ctx, "an", classroom.ID, roomID, true); err != nil {
		t.Fatal(err)
	}
	reason := "  Disruptive  "
	removed, err := svc.RemoveParticipant(ctx, "teacher", classroom.ID, roomID, "an", &reason)
	if err != nil || removed.RemovedAt == nil || removed.HandRaisedAt != nil || removed.ScreenShareApprovedAt != nil {
		t.Fatalf("remove = %#v err=%v", removed, err)
	}
	if !slices.Contains(mediaClient.removed, media.RoomName(roomID)+":an") {
		t.Fatalf("participant must be disconnected from LiveKit: %v", mediaClient.removed)
	}
	last := audits()[len(audits())-1]
	if last.Action != model.RoomModerationAuditActionRemove || last.Reason == nil || *last.Reason != "Disruptive" {
		t.Fatalf("remove audit = %#v", last)
	}
	if got := takeEvents(); !slices.Contains(got, sentRoomEvent{recipient: "an", eventType: learningSocket.EventParticipantRemoved, userID: "an"}) {
		t.Fatalf("remove events = %#v", got)
	}
	if _, err := svc.JoinRoom(ctx, "an", roomID); !errors.Is(err, service.ErrParticipantRemoved) {
		t.Fatalf("rejoin after removal = %v, want removed", err)
	}
	if _, err := svc.RequestScreenShare(ctx, "an", classroom.ID, roomID); !errors.Is(err, service.ErrParticipantRemoved) {
		t.Fatalf("screen share after removal = %v, want removed", err)
	}
	if _, err := svc.SetHandRaised(ctx, "an", classroom.ID, roomID, true); !errors.Is(err, service.ErrClassroomForbidden) {
		t.Fatalf("hand after removal = %v, want forbidden", err)
	}
	if _, err := svc.AllowParticipantMicrophone(ctx, "teacher", classroom.ID, roomID, "an"); !errors.Is(err, service.ErrParticipantRemoved) {
		t.Fatalf("moderating a removed participant = %v, want removed", err)
	}
	auditsBefore = len(audits())
	if _, err := svc.RemoveParticipant(ctx, "teacher", classroom.ID, roomID, "an", nil); err != nil || len(audits()) != auditsBefore {
		t.Fatalf("repeated remove must be a no-op: err=%v", err)
	}
	if activity, err := svc.GetRoomActivity(ctx, "an", classroom.ID, roomID); err != nil || activity.Participants[0].RemovedAt == nil {
		t.Fatalf("removed student must see the removal: %#v err=%v", activity, err)
	}
	if isMember, err := repo.IsStudentApprovedInClassroom(ctx, classroom.ID, "an"); err != nil || !isMember {
		t.Fatalf("removal from the room must not remove the classroom membership: %v err=%v", isMember, err)
	}

	// Kết thúc room dọn state nhưng giữ audit.
	auditsBefore = len(audits())
	if _, err := svc.EndRoom(ctx, "teacher", roomID); err != nil {
		t.Fatal(err)
	}
	if len(audits()) != auditsBefore {
		t.Fatal("audits must survive the end of the room")
	}
	if _, err := svc.MuteParticipant(ctx, "teacher", classroom.ID, roomID, "binh"); !errors.Is(err, service.ErrRoomEnded) {
		t.Fatalf("moderating an ended room = %v, want ended", err)
	}
}

// Mute All không dừng giữa chừng khi LiveKit lỗi ở một học sinh; học sinh bị mời ra không
// vào lại được bằng token còn hạn.
func TestMuteAllPartialFailureAndRemovedRejoin(t *testing.T) {
	db := learningTestDB(t)
	ctx, now := context.Background(), time.Now()
	classroom := &model.Classroom{Code: "MOD002", Name: "Mute all", TeacherID: "teacher", Active: true}
	if err := db.Create(classroom).Error; err != nil {
		t.Fatal(err)
	}
	approved, joinedAt := true, now
	students := []string{"student-a", "student-b", "student-c"}
	for _, student := range students {
		if err := db.Create(&model.Member{ClassroomID: classroom.ID, UserID: student, Approved: &approved, JoinedAt: &joinedAt}).Error; err != nil {
			t.Fatal(err)
		}
	}
	repo := repository.NewRepository(db)
	mediaClient := &fakeRoomMedia{created: map[string]bool{}, participants: map[string][]string{}, disconnected: map[string]bool{}, failPermissions: map[string]bool{"student-a": true}}
	svc := service.New(repo, &config.AppConfig{}, nil, testIdentityClient{}, nil, mediaClient, nil, nil)
	created, err := svc.CreateSchedule(ctx, "teacher", classroom.ID, dto.CreateScheduleRequest{Title: "Mute", StartsAt: now.Add(-time.Minute), EndsAt: now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	roomID := created.Room.ID
	if _, err := svc.StartRoom(ctx, "teacher", roomID); err != nil {
		t.Fatal(err)
	}
	for _, student := range students {
		if _, err := repo.GetOrCreateRoomParticipantState(ctx, roomID, student); err != nil {
			t.Fatal(err)
		}
	}

	result, err := svc.MuteAllParticipants(ctx, "teacher", classroom.ID, roomID)
	if err != nil || len(result.UserIDs) != len(students) {
		t.Fatalf("mute all must continue past a media error: result=%#v err=%v", result, err)
	}
	for _, student := range students[1:] {
		if got := mediaClient.permissions[student]; got.Microphone {
			t.Fatalf("%s must be muted in LiveKit: %#v", student, got)
		}
	}

	roomName := media.RoomName(roomID)
	if _, err := svc.RemoveParticipant(ctx, "teacher", classroom.ID, roomID, "student-b", nil); err != nil {
		t.Fatal(err)
	}
	kicks := len(mediaClient.removed)
	if err := svc.HandleParticipantJoined(ctx, roomName, "student-b", now); err != nil {
		t.Fatal(err)
	}
	if len(mediaClient.removed) != kicks+1 {
		t.Fatalf("removed student rejoining must be kicked: removed=%v", mediaClient.removed)
	}
	var open int64
	if err := db.Model(&model.RoomParticipant{}).Where("room_id = ? AND user_id = ? AND left_at IS NULL", roomID, "student-b").Count(&open).Error; err != nil || open != 0 {
		t.Fatalf("removed student must not get an attendance session: count=%d err=%v", open, err)
	}

	// Webhook bị mất, học sinh bị mời ra vẫn kết nối: worker đối chiếu đá ra, không mở phiên.
	mediaClient.participants[roomName] = []string{"teacher", "student-b"}
	kicks = len(mediaClient.removed)
	if _, err := svc.ProcessRoomLifecycle(ctx); err != nil {
		t.Fatal(err)
	}
	if len(mediaClient.removed) != kicks+1 {
		t.Fatalf("reconcile must kick the removed student: removed=%v", mediaClient.removed)
	}
	if err := db.Model(&model.RoomParticipant{}).Where("room_id = ? AND user_id = ? AND left_at IS NULL", roomID, "student-b").Count(&open).Error; err != nil || open != 0 {
		t.Fatalf("reconcile must not open a session for the removed student: count=%d err=%v", open, err)
	}
}
