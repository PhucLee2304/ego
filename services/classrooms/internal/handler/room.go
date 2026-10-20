package handler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"ego/platform/httpx"
	"ego/platform/jwt"
	"ego/services/classrooms/internal/dto"
	"ego/services/classrooms/internal/service"

	"github.com/livekit/protocol/auth"
	"github.com/livekit/protocol/livekit"
	"github.com/livekit/protocol/webhook"
	"gorm.io/gorm"
)

func roomID(w http.ResponseWriter, r *http.Request) (string, uint, bool) {
	userID, ok := jwt.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, httpx.NewError(http.StatusUnauthorized, "[UNAUTHORIZED] User ID not found", nil))
		return "", 0, false
	}
	id, err := httpx.ParsePathID(r, "roomId")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid room ID", err))
		return "", 0, false
	}
	return userID, id, true
}

func (h *handler) GetRoom(w http.ResponseWriter, r *http.Request) {
	userID, id, ok := roomID(w, r)
	if !ok {
		return
	}
	response, httpErr := h.service.GetRoom(r.Context(), userID, id)
	if httpErr != nil {
		httpx.Error(w, httpErr)
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
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid classroom ID", err))
		return
	}
	response, httpErr := h.service.GetRoomActivity(r.Context(), userID, classroomID, id)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}
func (h *handler) StartRoom(w http.ResponseWriter, r *http.Request) {
	userID, id, ok := roomID(w, r)
	if !ok {
		return
	}
	response, httpErr := h.service.StartRoom(r.Context(), userID, id)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}
func (h *handler) JoinRoom(w http.ResponseWriter, r *http.Request) {
	userID, id, ok := roomID(w, r)
	if !ok {
		return
	}
	response, httpErr := h.service.JoinRoom(r.Context(), userID, id)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}
func (h *handler) EndRoom(w http.ResponseWriter, r *http.Request) {
	userID, id, ok := roomID(w, r)
	if !ok {
		return
	}
	response, httpErr := h.service.EndRoom(r.Context(), userID, id)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}
func (h *handler) GetRoomAttendance(w http.ResponseWriter, r *http.Request) {
	userID, id, ok := roomID(w, r)
	if !ok {
		return
	}
	response, httpErr := h.service.GetRoomAttendance(r.Context(), userID, id)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}

// roomRequest đọc user từ JWT cùng classroom/room ID trên path. userId trên
// path chỉ là participant bị tác động; danh tính người gọi luôn lấy từ JWT.
func roomRequest(w http.ResponseWriter, r *http.Request) (string, uint, uint, bool) {
	userID, id, ok := roomID(w, r)
	if !ok {
		return "", 0, 0, false
	}
	classroomID, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid classroom ID", err))
		return "", 0, 0, false
	}
	return userID, classroomID, id, true
}

func (h *handler) RequestScreenShare(w http.ResponseWriter, r *http.Request) {
	userID, classroomID, id, ok := roomRequest(w, r)
	if !ok {
		return
	}
	response, httpErr := h.service.RequestScreenShare(r.Context(), userID, classroomID, id)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}

func (h *handler) ApproveScreenShare(w http.ResponseWriter, r *http.Request) {
	userID, classroomID, id, ok := roomRequest(w, r)
	if !ok {
		return
	}
	response, httpErr := h.service.ApproveScreenShare(r.Context(), userID, classroomID, id, r.PathValue("userId"))
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}

func (h *handler) RejectScreenShare(w http.ResponseWriter, r *http.Request) {
	userID, classroomID, id, ok := roomRequest(w, r)
	if !ok {
		return
	}
	response, httpErr := h.service.RejectScreenShare(r.Context(), userID, classroomID, id, r.PathValue("userId"))
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}

// StartScreenShare được client gọi ngay trước khi publish screen share.
func (h *handler) StartScreenShare(w http.ResponseWriter, r *http.Request) {
	userID, classroomID, id, ok := roomRequest(w, r)
	if !ok {
		return
	}
	if httpErr := h.service.StartScreenShare(r.Context(), userID, classroomID, id); httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) StopScreenShare(w http.ResponseWriter, r *http.Request) {
	userID, classroomID, id, ok := roomRequest(w, r)
	if !ok {
		return
	}
	if httpErr := h.service.StopScreenShare(r.Context(), userID, classroomID, id, r.PathValue("userId")); httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) SetHandRaised(w http.ResponseWriter, r *http.Request) {
	userID, classroomID, id, ok := roomRequest(w, r)
	if !ok {
		return
	}
	var req dto.UpdateHandRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid request body", err))
		return
	}
	response, httpErr := h.service.SetHandRaised(r.Context(), userID, classroomID, id, *req.Raised)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}

// LowerHands hạ tay toàn bộ học sinh, hoặc một học sinh khi path có userId.
func (h *handler) LowerHands(w http.ResponseWriter, r *http.Request) {
	userID, classroomID, id, ok := roomRequest(w, r)
	if !ok {
		return
	}
	var target *string
	if value := r.PathValue("userId"); value != "" {
		target = &value
	}
	response, httpErr := h.service.LowerHands(r.Context(), userID, classroomID, id, target)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}

type participantModeration func(ctx context.Context, teacherID string, classroomID, roomID uint, userID string) (*dto.RoomParticipantStateResponse, *httpx.HTTPError)

func (h *handler) moderateParticipant(action func(*service.HTTPService) participantModeration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, classroomID, id, ok := roomRequest(w, r)
		if !ok {
			return
		}
		response, httpErr := action(h.service)(r.Context(), userID, classroomID, id, r.PathValue("userId"))
		if httpErr != nil {
			httpx.Error(w, httpErr)
			return
		}
		httpx.JSON(w, http.StatusOK, response)
	}
}

func (h *handler) MuteParticipant(w http.ResponseWriter, r *http.Request) {
	h.moderateParticipant(func(s *service.HTTPService) participantModeration { return s.MuteParticipant })(w, r)
}

func (h *handler) AllowParticipantMicrophone(w http.ResponseWriter, r *http.Request) {
	h.moderateParticipant(func(s *service.HTTPService) participantModeration { return s.AllowParticipantMicrophone })(w, r)
}

func (h *handler) DisableParticipantCamera(w http.ResponseWriter, r *http.Request) {
	h.moderateParticipant(func(s *service.HTTPService) participantModeration { return s.DisableParticipantCamera })(w, r)
}

func (h *handler) AllowParticipantCamera(w http.ResponseWriter, r *http.Request) {
	h.moderateParticipant(func(s *service.HTTPService) participantModeration { return s.AllowParticipantCamera })(w, r)
}

func (h *handler) MuteAllParticipants(w http.ResponseWriter, r *http.Request) {
	userID, classroomID, id, ok := roomRequest(w, r)
	if !ok {
		return
	}
	response, httpErr := h.service.MuteAllParticipants(r.Context(), userID, classroomID, id)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}

// RemoveParticipant nhận body tùy chọn {"reason": "..."}; body rỗng hợp lệ.
func (h *handler) RemoveParticipant(w http.ResponseWriter, r *http.Request) {
	userID, classroomID, id, ok := roomRequest(w, r)
	if !ok {
		return
	}
	var req dto.RemoveParticipantRequest
	if err := httpx.DecodeJSON(r, &req); err != nil && !errors.Is(err, io.EOF) {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid request body", err))
		return
	}
	response, httpErr := h.service.RemoveParticipant(r.Context(), userID, classroomID, id, r.PathValue("userId"), req.Reason)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}

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
