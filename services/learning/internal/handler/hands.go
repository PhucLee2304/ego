package handler

import (
	"net/http"

	"ego/platform/httpx"
	"ego/services/learning/internal/dto"
)

// SetHandRaised godoc
// @Summary      Raise or lower own hand (student)
// @Tags         Rooms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  int  true  "Classroom ID"
// @Param        roomId  path  int  true  "Room ID"
// @Param        body  body  dto.UpdateHandRequest  true  "Request body"
// @Success      200  {object}  dto.RoomParticipantStateResponse
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      403  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Failure      409  {object}  httpx.HTTPError
// @Failure      503  {object}  httpx.HTTPError
// @Router       /classrooms/{id}/rooms/{roomId}/hand [patch]
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
//
// LowerHands godoc
// @Summary      Lower all raised hands (teacher)
// @Tags         Rooms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  int  true  "Classroom ID"
// @Param        roomId  path  int  true  "Room ID"
// @Success      200  {object}  dto.LoweredHandsResponse
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      403  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Failure      409  {object}  httpx.HTTPError
// @Failure      503  {object}  httpx.HTTPError
// @Router       /classrooms/{id}/rooms/{roomId}/hands [delete]
// @Router       /classrooms/{id}/rooms/{roomId}/hands/{userId} [delete]
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
