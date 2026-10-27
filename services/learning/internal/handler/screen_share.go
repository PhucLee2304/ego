package handler

import (
	"net/http"

	"ego/platform/httpx"

	// dto chỉ dùng trong Swagger godoc.
	_ "ego/services/learning/internal/dto"
)

// RequestScreenShare godoc
// @Summary      Request screen share permission (student)
// @Tags         Rooms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  int  true  "Classroom ID"
// @Param        roomId  path  int  true  "Room ID"
// @Success      200  {object}  dto.RoomParticipantStateResponse
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      403  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Failure      409  {object}  httpx.HTTPError
// @Failure      503  {object}  httpx.HTTPError
// @Router       /classrooms/{id}/rooms/{roomId}/screen-share/request [post]
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

// ApproveScreenShare godoc
// @Summary      Approve a screen share request (teacher)
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
// @Router       /classrooms/{id}/rooms/{roomId}/screen-share/{userId}/approve [post]
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

// RejectScreenShare godoc
// @Summary      Reject or revoke screen share (teacher)
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
// @Router       /classrooms/{id}/rooms/{roomId}/screen-share/{userId}/reject [post]
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
//
// StartScreenShare godoc
// @Summary      Start screen share; stops the current sharer
// @Tags         Rooms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  int  true  "Classroom ID"
// @Param        roomId  path  int  true  "Room ID"
// @Success      204
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      403  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Failure      409  {object}  httpx.HTTPError
// @Failure      503  {object}  httpx.HTTPError
// @Router       /classrooms/{id}/rooms/{roomId}/screen-share/start [post]
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

// StopScreenShare godoc
// @Summary      Stop screen share
// @Tags         Rooms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  int  true  "Classroom ID"
// @Param        roomId  path  int  true  "Room ID"
// @Param        userId  path  string  true  "Participant user ID"
// @Success      204
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      403  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Failure      409  {object}  httpx.HTTPError
// @Failure      503  {object}  httpx.HTTPError
// @Router       /classrooms/{id}/rooms/{roomId}/screen-share/{userId}/stop [post]
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
