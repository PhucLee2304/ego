//go:build integration

package integration_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"testing"
	"time"

	"ego/services/classrooms/internal/media"

	"github.com/livekit/protocol/livekit"
	lksdk "github.com/livekit/server-sdk-go/v2"
)

// Kiểm tra với LiveKit thật rằng quyền screen share được cấp/thu hồi trên phiên
// đang kết nối và participant vắng mặt được phân biệt với lỗi media server.
// Môi trường test không gửi media từ client Go nên không publish track ở đây.
func TestLiveKitScreenSharePermissionUpdates(t *testing.T) {
	url := os.Getenv("LIVEKIT_TEST_URL")
	apiKey := os.Getenv("LIVEKIT_TEST_API_KEY")
	apiSecret := os.Getenv("LIVEKIT_TEST_API_SECRET")
	if url == "" || apiKey == "" || apiSecret == "" {
		t.Skip("LiveKit integration test is not configured")
	}
	ctx := context.Background()
	client := media.New(url, apiKey, apiSecret, time.Minute)
	roomName := fmt.Sprintf("screen-share-test-%d", time.Now().UnixNano())
	if err := client.CreateRoom(ctx, roomName); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.DeleteRoom(context.Background(), roomName) })

	token, err := client.CreateJoinToken(roomName, "student-1", "Student One", false, media.PublishPermissions{Camera: true, Microphone: true})
	if err != nil {
		t.Fatal(err)
	}
	room, err := lksdk.ConnectToRoomWithToken(url, token, &lksdk.RoomCallback{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(room.Disconnect)

	canShare := func() bool {
		return slices.Contains(room.LocalParticipant.Permissions().GetCanPublishSources(), livekit.TrackSource_SCREEN_SHARE)
	}
	waitFor := func(description string, done func() bool) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for !done() {
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for %s", description)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}

	if canShare() {
		t.Fatal("a student token must not allow screen sharing before approval")
	}
	if err := client.SetPublishPermissions(ctx, roomName, "student-1", media.PublishPermissions{Camera: true, Microphone: true, ScreenShare: true}); err != nil {
		t.Fatal(err)
	}
	waitFor("approval to reach the participant", canShare)
	sources := room.LocalParticipant.Permissions().GetCanPublishSources()
	if !slices.Contains(sources, livekit.TrackSource_CAMERA) || !slices.Contains(sources, livekit.TrackSource_MICROPHONE) {
		t.Fatalf("approval must keep camera and microphone: %v", sources)
	}
	if !room.LocalParticipant.Permissions().GetCanPublishData() {
		t.Fatal("approval must keep data publishing for reactions")
	}

	if sharers, err := client.ListScreenSharers(ctx, roomName); err != nil || len(sharers) != 0 {
		t.Fatalf("sharers = %v, err=%v", sharers, err)
	}
	if err := client.MuteTracks(ctx, roomName, "student-1", media.SourceScreenShare); err != nil {
		t.Fatalf("stopping a participant without a screen share must succeed: %v", err)
	}
	if err := client.SetPublishPermissions(ctx, roomName, "student-1", media.PublishPermissions{Camera: true, Microphone: true}); err != nil {
		t.Fatal(err)
	}
	waitFor("revocation to reach the participant", func() bool { return !canShare() })

	// Tắt cả mic lẫn camera: không được để LiveKit hiểu thành "mọi nguồn".
	if err := client.SetPublishPermissions(ctx, roomName, "student-1", media.PublishPermissions{}); err != nil {
		t.Fatal(err)
	}
	waitFor("all publishing to be revoked", func() bool {
		permission := room.LocalParticipant.Permissions()
		return !permission.GetCanPublish() && permission.GetCanSubscribe() && permission.GetCanPublishData()
	})

	if err := client.RemoveParticipant(ctx, roomName, "student-1"); err != nil {
		t.Fatal(err)
	}
	waitFor("removed participant to leave the room", func() bool {
		identities, err := client.ListParticipants(ctx, roomName)
		return err == nil && !slices.Contains(identities, "student-1")
	})
	if err := client.RemoveParticipant(ctx, roomName, "student-1"); !errors.Is(err, media.ErrParticipantNotFound) {
		t.Fatalf("removing a participant twice = %v, want ErrParticipantNotFound", err)
	}

	if err := client.SetPublishPermissions(ctx, roomName, "ghost", media.HostPermissions()); !errors.Is(err, media.ErrParticipantNotFound) {
		t.Fatalf("permission for a missing participant = %v, want ErrParticipantNotFound", err)
	}
	if err := client.DeleteRoom(ctx, "missing-"+roomName); err != nil {
		t.Fatalf("deleting a room that no longer exists must succeed: %v", err)
	}
	if err := client.MuteTracks(ctx, roomName, "ghost", media.SourceScreenShare); !errors.Is(err, media.ErrParticipantNotFound) {
		t.Fatalf("stop for a missing participant = %v, want ErrParticipantNotFound", err)
	}
}
