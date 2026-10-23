package service

import (
	"net/http"

	"ego/platform/httpx"
	"ego/services/learning/internal/media"
	"ego/services/learning/internal/model"
)

var ErrParticipantRemoved = httpx.NewError(http.StatusForbidden, "[FORBIDDEN] You were removed from this session by the teacher", nil)

// participantPermissions tính quyền publish của học sinh từ state; state nil
// nghĩa là chưa bị giới hạn gì.
func participantPermissions(state *model.RoomParticipantState, screenShare bool) media.PublishPermissions {
	if state == nil {
		return media.PublishPermissions{
			Camera:      true,
			Microphone:  true,
			ScreenShare: screenShare}
	}
	return media.PublishPermissions{
		Camera:      state.CameraAllowed,
		Microphone:  state.MicrophoneAllowed,
		ScreenShare: screenShare}
}
