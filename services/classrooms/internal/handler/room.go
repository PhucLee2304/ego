package handler

import (
	"errors"
	"net/http"
	"time"

	"ego/platform/httpx"
	"ego/platform/jwt"
	"ego/services/classrooms/internal/service"

	"github.com/livekit/protocol/auth"
	"github.com/livekit/protocol/webhook"
	"gorm.io/gorm"
)

func roomID(w http.ResponseWriter, r *http.Request) (string, uint, bool) {
	userID, ok := jwt.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "[UNAUTHORIZED] User ID not found")
		return "", 0, false
	}
	id, err := httpx.ParsePathID(r, "roomId")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid room ID")
		return "", 0, false
	}
	return userID, id, true
}

func writeRoomError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrClassroomForbidden):
		httpx.Error(w, http.StatusForbidden, err.Error())
	case errors.Is(err, gorm.ErrRecordNotFound):
		httpx.Error(w, http.StatusNotFound, "[ERROR] Resource not found")
	case errors.Is(err, service.ErrRoomLive):
		httpx.Error(w, http.StatusConflict, err.Error())
	case errors.Is(err, service.ErrRoomNotLive), errors.Is(err, service.ErrRoomEnded), errors.Is(err, service.ErrRoomTooEarly), errors.Is(err, service.ErrClassroomNotActive):
		httpx.Error(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrMediaUnavailable):
		httpx.Error(w, http.StatusServiceUnavailable, err.Error())
	default:
		httpx.Error(w, http.StatusInternalServerError, err.Error())
	}
}

func (h *handler) GetRoom(w http.ResponseWriter, r *http.Request) {
	userID, id, ok := roomID(w, r)
	if !ok {
		return
	}
	response, err := h.service.GetRoom(r.Context(), userID, id)
	if err != nil {
		writeRoomError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}

func (h *handler) GetRoomActivity(w http.ResponseWriter, r *http.Request) {
	userID, id, ok := roomID(w, r)
	if !ok {
		return
	}
	classroomID, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid classroom ID")
		return
	}
	response, err := h.service.GetRoomActivity(r.Context(), userID, classroomID, id)
	if err != nil {
		writeRoomError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}
func (h *handler) StartRoom(w http.ResponseWriter, r *http.Request) {
	userID, id, ok := roomID(w, r)
	if !ok {
		return
	}
	response, err := h.service.StartRoom(r.Context(), userID, id)
	if err != nil {
		writeRoomError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}
func (h *handler) JoinRoom(w http.ResponseWriter, r *http.Request) {
	userID, id, ok := roomID(w, r)
	if !ok {
		return
	}
	response, err := h.service.JoinRoom(r.Context(), userID, id)
	if err != nil {
		writeRoomError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}
func (h *handler) EndRoom(w http.ResponseWriter, r *http.Request) {
	userID, id, ok := roomID(w, r)
	if !ok {
		return
	}
	response, err := h.service.EndRoom(r.Context(), userID, id)
	if err != nil {
		writeRoomError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}
func (h *handler) GetRoomAttendance(w http.ResponseWriter, r *http.Request) {
	userID, id, ok := roomID(w, r)
	if !ok {
		return
	}
	response, err := h.service.GetRoomAttendance(r.Context(), userID, id)
	if err != nil {
		writeRoomError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}

func LiveKitWebhook(svc service.Service, apiKey, apiSecret string) http.HandlerFunc {
	provider := auth.NewSimpleKeyProvider(apiKey, apiSecret)
	return func(w http.ResponseWriter, r *http.Request) {
		event, err := webhook.ReceiveWebhookEvent(r, provider)
		if err != nil {
			httpx.Error(w, http.StatusUnauthorized, "[UNAUTHORIZED] Invalid webhook signature")
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
		}
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			httpx.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
