package handler

import (
	"errors"
	"net/http"
	"time"

	"ego/platform/httpx"
	"ego/services/learning/internal/service"

	"github.com/livekit/protocol/auth"
	"github.com/livekit/protocol/livekit"
	"github.com/livekit/protocol/webhook"
	"gorm.io/gorm"
)

// LiveKitWebhook nhận webhook LiveKit (xác thực chữ ký bằng API key/secret).
// Không qua JWT nên mount riêng ở main.go, không nằm trong RegisterRoutes.
func LiveKitWebhook(svc service.Service, apiKey, apiSecret string) http.HandlerFunc {
	provider := auth.NewSimpleKeyProvider(apiKey, apiSecret)
	return func(w http.ResponseWriter, r *http.Request) {
		event, err := webhook.ReceiveWebhookEvent(r, provider)
		if err != nil {
			httpx.Error(w, httpx.NewError(http.StatusUnauthorized, "[UNAUTHORIZED] Invalid webhook signature", nil))
			return
		}
		when := time.Now()
		switch event.Event {
		case "participant_joined":
			if event.Room != nil && event.Participant != nil {
				err = svc.HandleParticipantJoined(r.Context(), event.Room.Name, event.Participant.Identity, when)
			}
		case "participant_left":
			if event.Room != nil && event.Participant != nil {
				err = svc.HandleParticipantLeft(r.Context(), event.Room.Name, event.Participant.Identity, when)
			}
		case "track_published", "track_unpublished":
			if event.Room != nil && event.Participant != nil && event.Track != nil && event.Track.Source == livekit.TrackSource_SCREEN_SHARE {
				if event.Event == "track_published" {
					err = svc.HandleScreenSharePublished(r.Context(), event.Room.Name, event.Participant.Identity)
				} else {
					err = svc.HandleScreenShareUnpublished(r.Context(), event.Room.Name, event.Participant.Identity)
				}
			}
		}
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			httpx.Error(w, httpx.NewError(http.StatusInternalServerError, err.Error(), err))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
