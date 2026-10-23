package handler

import (
	"context"
	"errors"
	"io"
	"net/http"

	"ego/platform/httpx"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/service"
)

type participantModeration func(ctx context.Context, teacherID string, classroomID, roomID uint, userID string) (*dto.RoomParticipantStateResponse, *httpx.HTTPError)

func (h *handler) moderateParticipant(action func(service.Service) participantModeration) http.HandlerFunc {
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

// MuteParticipant godoc
// @Summary      Mute a student microphone (teacher)
// @Tags         Rooms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  int  true  "Classroom ID"
// @Param        roomId  path  int  true  "Room ID"
// @Param        userId  path  string  true  "Participant user ID"
// @Success      200  {object}  dto.RoomParticipantStateResponse
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      403  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Failure      409  {object}  httpx.HTTPError
// @Failure      503  {object}  httpx.HTTPError
// @Router       /classrooms/{id}/rooms/{roomId}/participants/{userId}/mute [post]
func (h *handler) MuteParticipant(w http.ResponseWriter, r *http.Request) {
	h.moderateParticipant(func(s service.Service) participantModeration { return s.MuteParticipant })(w, r)
}

// AllowParticipantMicrophone godoc
// @Summary      Allow a student to use the microphone again (teacher)
// @Tags         Rooms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  int  true  "Classroom ID"
// @Param        roomId  path  int  true  "Room ID"
// @Param        userId  path  string  true  "Participant user ID"
// @Success      200  {object}  dto.RoomParticipantStateResponse
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      403  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Failure      409  {object}  httpx.HTTPError
// @Failure      503  {object}  httpx.HTTPError
// @Router       /classrooms/{id}/rooms/{roomId}/participants/{userId}/allow-microphone [post]
func (h *handler) AllowParticipantMicrophone(w http.ResponseWriter, r *http.Request) {
	h.moderateParticipant(func(s service.Service) participantModeration { return s.AllowParticipantMicrophone })(w, r)
}

// DisableParticipantCamera godoc
// @Summary      Disable a student camera (teacher)
// @Tags         Rooms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  int  true  "Classroom ID"
// @Param        roomId  path  int  true  "Room ID"
// @Param        userId  path  string  true  "Participant user ID"
// @Success      200  {object}  dto.RoomParticipantStateResponse
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      403  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Failure      409  {object}  httpx.HTTPError
// @Failure      503  {object}  httpx.HTTPError
// @Router       /classrooms/{id}/rooms/{roomId}/participants/{userId}/disable-camera [post]
func (h *handler) DisableParticipantCamera(w http.ResponseWriter, r *http.Request) {
	h.moderateParticipant(func(s service.Service) participantModeration { return s.DisableParticipantCamera })(w, r)
}

// AllowParticipantCamera godoc
// @Summary      Allow a student to use the camera again (teacher)
// @Tags         Rooms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  int  true  "Classroom ID"
// @Param        roomId  path  int  true  "Room ID"
// @Param        userId  path  string  true  "Participant user ID"
// @Success      200  {object}  dto.RoomParticipantStateResponse
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      403  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Failure      409  {object}  httpx.HTTPError
// @Failure      503  {object}  httpx.HTTPError
// @Router       /classrooms/{id}/rooms/{roomId}/participants/{userId}/allow-camera [post]
func (h *handler) AllowParticipantCamera(w http.ResponseWriter, r *http.Request) {
	h.moderateParticipant(func(s service.Service) participantModeration { return s.AllowParticipantCamera })(w, r)
}

// MuteAllParticipants godoc
// @Summary      Mute all students (teacher)
// @Tags         Rooms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  int  true  "Classroom ID"
// @Param        roomId  path  int  true  "Room ID"
// @Success      200  {object}  dto.MutedParticipantsResponse
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      403  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Failure      409  {object}  httpx.HTTPError
// @Failure      503  {object}  httpx.HTTPError
// @Router       /classrooms/{id}/rooms/{roomId}/participants/mute-all [post]
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
//
// RemoveParticipant godoc
// @Summary      Remove a student from the session until it ends (teacher)
// @Tags         Rooms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  int  true  "Classroom ID"
// @Param        roomId  path  int  true  "Room ID"
// @Param        userId  path  string  true  "Participant user ID"
// @Param        body  body  dto.RemoveParticipantRequest  false  "Request body"
// @Success      200  {object}  dto.RoomParticipantStateResponse
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      403  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Failure      409  {object}  httpx.HTTPError
// @Failure      503  {object}  httpx.HTTPError
// @Router       /classrooms/{id}/rooms/{roomId}/participants/{userId}/remove [post]
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
