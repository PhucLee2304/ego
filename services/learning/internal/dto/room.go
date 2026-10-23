package dto

import (
	"time"

	"ego/services/learning/internal/model"
)

type RoomResponse struct {
	ID          uint             `json:"id"`
	ClassroomID uint             `json:"classroomId"`
	ScheduleID  uint             `json:"scheduleId"`
	Status      model.RoomStatus `json:"status"`
	StartedAt   *time.Time       `json:"startedAt,omitempty"`
	EndedAt     *time.Time       `json:"endedAt,omitempty"`
	StartsAt    time.Time        `json:"startsAt"`
	EndsAt      time.Time        `json:"endsAt"`
	IsHost      bool             `json:"isHost"`
}

type JoinRoomResponse struct {
	Room  *RoomResponse `json:"room"`
	URL   string        `json:"url"`
	Token string        `json:"token"`
}

type UpdateHandRequest struct {
	Raised *bool `json:"raised" validate:"required"`
}

type MutedParticipantsResponse struct {
	UserIDs []string `json:"userIds"`
}

type RemoveParticipantRequest struct {
	Reason *string `json:"reason,omitempty" validate:"omitempty,max=500"`
}

type LoweredHandsResponse struct {
	UserIDs []string `json:"userIds"`
}

type RoomParticipantStateResponse struct {
	UserID                 string     `json:"userId"`
	MicrophoneAllowed      bool       `json:"microphoneAllowed"`
	CameraAllowed          bool       `json:"cameraAllowed"`
	HandRaisedAt           *time.Time `json:"handRaisedAt,omitempty"`
	RemovedAt              *time.Time `json:"removedAt,omitempty"`
	ScreenShareRequestedAt *time.Time `json:"screenShareRequestedAt,omitempty"`
	ScreenShareApprovedAt  *time.Time `json:"screenShareApprovedAt,omitempty"`
	UpdatedAt              time.Time  `json:"updatedAt"`
}

type RoomActivityResponse struct {
	RoomID       uint                            `json:"roomId"`
	ClassroomID  uint                            `json:"classroomId"`
	Status       model.RoomStatus                `json:"status"`
	IsHost       bool                            `json:"isHost"`
	Participants []*RoomParticipantStateResponse `json:"participants"`
	UpdatedAt    time.Time                       `json:"updatedAt"`
}

type AttendanceSessionResponse struct {
	ID       uint                      `json:"id"`
	User     *UserInfo                 `json:"user,omitempty"`
	Role     model.RoomParticipantRole `json:"role"`
	JoinedAt time.Time                 `json:"joinedAt"`
	LeftAt   *time.Time                `json:"leftAt,omitempty"`
}

func ToRoomResponse(room *model.Room, isHost bool) *RoomResponse {
	return &RoomResponse{
		ID:          room.ID,
		ClassroomID: room.Schedule.ClassroomID,
		ScheduleID:  room.ScheduleID,
		Status:      room.Status,
		StartedAt:   room.StartedAt,
		EndedAt:     room.EndedAt,
		StartsAt:    room.Schedule.StartsAt,
		EndsAt:      room.Schedule.EndsAt,
		IsHost:      isHost,
	}
}

func ToRoomParticipantStateResponse(state *model.RoomParticipantState) *RoomParticipantStateResponse {
	return &RoomParticipantStateResponse{
		UserID:                 state.UserID,
		MicrophoneAllowed:      state.MicrophoneAllowed,
		CameraAllowed:          state.CameraAllowed,
		HandRaisedAt:           state.HandRaisedAt,
		RemovedAt:              state.RemovedAt,
		ScreenShareRequestedAt: state.ScreenShareRequestedAt,
		ScreenShareApprovedAt:  state.ScreenShareApprovedAt,
		UpdatedAt:              state.UpdatedAt,
	}
}

func ToRoomActivityResponse(room *model.Room, states []*model.RoomParticipantState, userID string, isHost bool) *RoomActivityResponse {
	participants := make([]*RoomParticipantStateResponse, 0, len(states))
	updatedAt := room.UpdatedAt
	for _, state := range states {
		if !isHost && state.UserID != userID {
			continue
		}
		participants = append(participants, ToRoomParticipantStateResponse(state))
		if state.UpdatedAt.After(updatedAt) {
			updatedAt = state.UpdatedAt
		}
	}
	return &RoomActivityResponse{
		RoomID:       room.ID,
		ClassroomID:  room.Schedule.ClassroomID,
		Status:       room.Status,
		IsHost:       isHost,
		Participants: participants,
		UpdatedAt:    updatedAt,
	}
}
