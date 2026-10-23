package handler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ego/services/learning/internal/service"

	"github.com/livekit/protocol/auth"
	"github.com/livekit/protocol/livekit"
	"google.golang.org/protobuf/encoding/protojson"
)

const (
	webhookTestAPIKey    = "devkey"
	webhookTestAPISecret = "local-development-secret-key-000001"
)

type webhookService struct {
	service.Service
	joined      []string
	left        []string
	published   []string
	unpublished []string
}

func (s *webhookService) HandleScreenSharePublished(_ context.Context, room, userID string) error {
	s.published = append(s.published, room+":"+userID)
	return nil
}

func (s *webhookService) HandleScreenShareUnpublished(_ context.Context, room, userID string) error {
	s.unpublished = append(s.unpublished, room+":"+userID)
	return nil
}

func (s *webhookService) HandleParticipantJoined(_ context.Context, room, userID string, _ time.Time) error {
	s.joined = append(s.joined, room+":"+userID)
	return nil
}

func (s *webhookService) HandleParticipantLeft(_ context.Context, room, userID string, _ time.Time) error {
	s.left = append(s.left, room+":"+userID)
	return nil
}

func TestLiveKitWebhookVerifiesSignatureAndAcceptsDuplicateEvents(t *testing.T) {
	svc := &webhookService{}
	handler := LiveKitWebhook(svc, webhookTestAPIKey, webhookTestAPISecret)
	event := &livekit.WebhookEvent{
		Event:       "participant_joined",
		Room:        &livekit.Room{Name: "classroom-room-7"},
		Participant: &livekit.ParticipantInfo{Identity: "student-1"},
	}

	for range 2 {
		request := signedWebhookRequest(t, event, webhookTestAPISecret)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusNoContent, response.Body.String())
		}
	}
	if len(svc.joined) != 2 || svc.joined[0] != "classroom-room-7:student-1" {
		t.Fatalf("unexpected joined calls: %#v", svc.joined)
	}

	left := &livekit.WebhookEvent{
		Event:       "participant_left",
		Room:        event.Room,
		Participant: event.Participant,
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, signedWebhookRequest(t, left, webhookTestAPISecret))
	if response.Code != http.StatusNoContent || len(svc.left) != 1 {
		t.Fatalf("left webhook status=%d calls=%#v", response.Code, svc.left)
	}
}

func TestLiveKitWebhookRoutesOnlyScreenShareTrackEvents(t *testing.T) {
	svc := &webhookService{}
	handler := LiveKitWebhook(svc, webhookTestAPIKey, webhookTestAPISecret)
	room := &livekit.Room{Name: "classroom-room-7"}
	participant := &livekit.ParticipantInfo{Identity: "student-1"}
	for _, event := range []*livekit.WebhookEvent{
		{Event: "track_published", Room: room, Participant: participant, Track: &livekit.TrackInfo{Source: livekit.TrackSource_SCREEN_SHARE}},
		{Event: "track_published", Room: room, Participant: participant, Track: &livekit.TrackInfo{Source: livekit.TrackSource_CAMERA}},
		{Event: "track_unpublished", Room: room, Participant: participant, Track: &livekit.TrackInfo{Source: livekit.TrackSource_SCREEN_SHARE}},
		{Event: "track_unpublished", Room: room, Participant: participant, Track: &livekit.TrackInfo{Source: livekit.TrackSource_MICROPHONE}},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, signedWebhookRequest(t, event, webhookTestAPISecret))
		if response.Code != http.StatusNoContent {
			t.Fatalf("%s status = %d; body=%s", event.Event, response.Code, response.Body.String())
		}
	}
	if len(svc.published) != 1 || svc.published[0] != "classroom-room-7:student-1" || len(svc.unpublished) != 1 {
		t.Fatalf("published=%#v unpublished=%#v", svc.published, svc.unpublished)
	}
}

func TestLiveKitWebhookRejectsInvalidSignature(t *testing.T) {
	svc := &webhookService{}
	event := &livekit.WebhookEvent{Event: "participant_joined"}
	request := signedWebhookRequest(t, event, "different-secret")
	response := httptest.NewRecorder()

	LiveKitWebhook(svc, webhookTestAPIKey, webhookTestAPISecret).ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	if strings.Contains(response.Body.String(), webhookTestAPIKey) || strings.Contains(response.Body.String(), webhookTestAPISecret) {
		t.Fatalf("authentication response exposed LiveKit credentials: %s", response.Body.String())
	}
	if len(svc.joined) != 0 {
		t.Fatalf("invalid webhook reached service: %#v", svc.joined)
	}
}

func signedWebhookRequest(t *testing.T, event *livekit.WebhookEvent, secret string) *http.Request {
	t.Helper()
	body, err := protojson.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	token, err := auth.NewAccessToken(webhookTestAPIKey, secret).
		SetValidFor(time.Minute).
		SetSha256(base64.StdEncoding.EncodeToString(sum[:])).
		ToJWT()
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/livekit/webhook", bytes.NewReader(body))
	request.Header.Set("Authorization", token)
	return request
}
