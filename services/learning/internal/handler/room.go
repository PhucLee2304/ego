package handler

import (
	"net/http"

	"ego/platform/httpx"
	"ego/platform/jwt"

	_ "ego/services/learning/internal/dto"
)

// GetRoom godoc
// @Summary      Get room
// @Tags         Rooms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  int  true  "Classroom ID"
// @Param        roomId  path  int  true  "Room ID"
// @Success      200  {object}  dto.RoomResponse
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      403  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Router       /classrooms/{id}/rooms/{roomId} [get]
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

// GetRoomActivity godoc
// @Summary      Get participant activity of a live room
// @Tags         Rooms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  int  true  "Classroom ID"
// @Param        roomId  path  int  true  "Room ID"
// @Success      200  {object}  dto.RoomActivityResponse
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      403  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Router       /classrooms/{id}/rooms/{roomId}/activity [get]
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

// StartRoom godoc
// @Summary      Start room (teacher)
// @Tags         Rooms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  int  true  "Classroom ID"
// @Param        roomId  path  int  true  "Room ID"
// @Success      200  {object}  dto.RoomResponse
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      403  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Failure      503  {object}  httpx.HTTPError
// @Router       /classrooms/{id}/rooms/{roomId}/start [post]
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

// JoinRoom godoc
// @Summary      Get a LiveKit join token for the room
// @Tags         Rooms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  int  true  "Classroom ID"
// @Param        roomId  path  int  true  "Room ID"
// @Success      200  {object}  dto.JoinRoomResponse
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      403  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Failure      503  {object}  httpx.HTTPError
// @Router       /classrooms/{id}/rooms/{roomId}/join [post]
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

// EndRoom godoc
// @Summary      End room (teacher)
// @Tags         Rooms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  int  true  "Classroom ID"
// @Param        roomId  path  int  true  "Room ID"
// @Success      200  {object}  dto.RoomResponse
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      403  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Failure      503  {object}  httpx.HTTPError
// @Router       /classrooms/{id}/rooms/{roomId}/end [post]
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

// GetRoomAttendance godoc
// @Summary      Get room attendance sessions
// @Tags         Rooms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  int  true  "Classroom ID"
// @Param        roomId  path  int  true  "Room ID"
// @Success      200  {array}  dto.AttendanceSessionResponse
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      403  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Router       /classrooms/{id}/rooms/{roomId}/attendance [get]
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

// roomID đọc user từ JWT và room ID trên path.
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
