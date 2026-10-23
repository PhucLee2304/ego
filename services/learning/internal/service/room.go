package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	identityClient "ego/api/gen/go/identity"
	"ego/platform/httpx"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/media"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"
)

func (s *service) GetRoom(ctx context.Context, userID string, roomID uint) (*dto.RoomResponse, *httpx.HTTPError) {
	room, err := s.repo.GetRoomByID(ctx, roomID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	isHost := room.Schedule.Classroom.TeacherID == userID
	if !isHost {
		approved, err := s.repo.IsStudentApprovedInClassroom(ctx, room.Schedule.ClassroomID, userID)
		if err != nil {
			return nil, httpx.FromError(err, errClassroomServiceFailed)
		}
		if !approved {
			return nil, ErrClassroomForbidden
		}
	}
	return dto.ToRoomResponse(room, isHost), nil
}

func (s *service) GetRoomActivity(ctx context.Context, userID string, classroomID uint, roomID uint) (*dto.RoomActivityResponse, *httpx.HTTPError) {
	room, err := s.repo.GetRoomByID(ctx, roomID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	if room.Schedule.ClassroomID != classroomID {
		return nil, ErrClassroomForbidden
	}
	isHost := room.Schedule.Classroom.TeacherID == userID
	if !isHost {
		approved, err := s.repo.IsStudentApprovedInClassroom(ctx, classroomID, userID)
		if err != nil {
			return nil, httpx.FromError(err, errClassroomServiceFailed)
		}
		if !approved {
			return nil, ErrClassroomForbidden
		}
	}
	if room.Status == model.RoomStatusEnded {
		return nil, ErrRoomEnded
	}
	states, err := s.repo.GetRoomParticipantStates(ctx, roomID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	return dto.ToRoomActivityResponse(room, states, userID, isHost), nil
}

func (s *service) StartRoom(ctx context.Context, teacherID string, roomID uint) (*dto.RoomResponse, *httpx.HTTPError) {
	if s.mediaClient == nil {
		return nil, ErrMediaUnavailable
	}
	room, err := s.repo.GetRoomByID(ctx, roomID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	if room.Schedule.Classroom.TeacherID != teacherID {
		return nil, ErrClassroomForbidden
	}
	if !room.Schedule.Classroom.Active {
		return nil, ErrClassroomNotActive
	}
	if room.Status == model.RoomStatusEnded {
		return nil, ErrRoomEnded
	}
	if room.Status == model.RoomStatusLive {
		return dto.ToRoomResponse(room, true), nil
	}
	if time.Now().Before(room.Schedule.StartsAt.Add(-30 * time.Minute)) {
		return nil, ErrRoomTooEarly
	}
	if !time.Now().Before(room.Schedule.EndsAt) {
		return nil, ErrRoomEnded
	}
	if err := s.mediaClient.CreateRoom(ctx, media.RoomName(room.ID)); err != nil {
		return nil, httpx.FromError(errors.Join(ErrMediaUnavailable, err), errClassroomServiceFailed)
	}
	now := time.Now()
	err = s.repo.Transaction(ctx, func(repo repository.Repository) error {
		locked, err := repo.GetRoomByIDForUpdate(ctx, roomID)
		if err != nil {
			return err
		}
		if locked.Status != model.RoomStatusScheduled {
			return ErrRoomEnded
		}
		locked.Status, locked.StartedAt, locked.HostAbsentSince = model.RoomStatusLive, &now, nil
		if err := repo.UpdateRoom(ctx, locked); err != nil {
			return err
		}
		members, err := repo.GetApprovedMembersByClassroom(ctx, locked.Schedule.ClassroomID)
		if err != nil {
			return err
		}
		recipients := make([]string, len(members))
		for i, member := range members {
			recipients[i] = member.UserID
		}
		if err := createNotificationOutbox(ctx, repo, model.NotificationSourceRoom, fmt.Sprintf("room:%d:started", roomID), recipients,
			"Class has started", locked.Schedule.Title+" is now live", "ROOM", roomID, locked.Schedule.ClassroomID, time.Time{}); err != nil {
			return err
		}
		room = locked
		return nil
	})
	if err != nil {
		_ = s.mediaClient.DeleteRoom(context.WithoutCancel(ctx), media.RoomName(roomID))
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	s.publishRoomState(ctx, room, "ROOM_STARTED")
	return dto.ToRoomResponse(room, true), nil
}

func (s *service) JoinRoom(ctx context.Context, userID string, roomID uint) (*dto.JoinRoomResponse, *httpx.HTTPError) {
	if s.mediaClient == nil {
		return nil, ErrMediaUnavailable
	}
	room, err := s.repo.GetRoomByID(ctx, roomID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	isHost := room.Schedule.Classroom.TeacherID == userID
	if !isHost {
		approved, err := s.repo.IsStudentApprovedInClassroom(ctx, room.Schedule.ClassroomID, userID)
		if err != nil {
			return nil, httpx.FromError(err, errClassroomServiceFailed)
		}
		if !approved {
			return nil, ErrClassroomForbidden
		}
	}
	if !room.Schedule.Classroom.Active {
		return nil, ErrClassroomNotActive
	}
	if room.Status == model.RoomStatusScheduled {
		return nil, ErrRoomNotLive
	}
	if room.Status == model.RoomStatusEnded {
		return nil, ErrRoomEnded
	}
	state, err := s.repo.GetOrCreateRoomParticipantState(ctx, room.ID, userID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	// Học sinh bị giáo viên xóa khỏi phòng không được vào lại cho đến khi room
	// kết thúc; state (cùng RemovedAt) chỉ bị xóa lúc đó.
	if state.RemovedAt != nil {
		return nil, ErrParticipantRemoved
	}
	permissions := media.HostPermissions()
	if !isHost {
		permissions = participantPermissions(state, false)
	}
	name := userID
	usersResp, err := s.identityClient.GetUsersInfo(ctx, &identityClient.GetUsersInfoRequest{Ids: []string{userID}})
	if err == nil && len(usersResp.Users) > 0 {
		name = usersResp.Users[0].Name
	}
	token, err := s.mediaClient.CreateJoinToken(media.RoomName(room.ID), userID, name, isHost, permissions)
	if err != nil {
		return nil, httpx.FromError(errors.Join(ErrMediaUnavailable, err), errClassroomServiceFailed)
	}
	return &dto.JoinRoomResponse{Room: dto.ToRoomResponse(room, isHost), URL: s.cfg.LiveKitPublicURL, Token: token}, nil
}

func (s *service) EndRoom(ctx context.Context, teacherID string, roomID uint) (*dto.RoomResponse, *httpx.HTTPError) {
	room, err := s.repo.GetRoomByID(ctx, roomID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	if room.Schedule.Classroom.TeacherID != teacherID {
		return nil, ErrClassroomForbidden
	}
	if room.Status == model.RoomStatusEnded {
		return dto.ToRoomResponse(room, true), nil
	}
	if room.Status != model.RoomStatusLive {
		return nil, ErrRoomNotLive
	}
	if s.mediaClient == nil {
		return nil, ErrMediaUnavailable
	}
	if err := s.mediaClient.DeleteRoom(ctx, media.RoomName(room.ID)); err != nil {
		return nil, httpx.FromError(errors.Join(ErrMediaUnavailable, err), errClassroomServiceFailed)
	}
	if err := s.finishRoom(ctx, room); err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	return dto.ToRoomResponse(room, true), nil
}

func (s *service) GetRoomAttendance(ctx context.Context, userID string, roomID uint) ([]*dto.AttendanceSessionResponse, *httpx.HTTPError) {
	room, err := s.repo.GetRoomByID(ctx, roomID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	isHost := room.Schedule.Classroom.TeacherID == userID
	if !isHost {
		approved, err := s.repo.IsStudentApprovedInClassroom(ctx, room.Schedule.ClassroomID, userID)
		if err != nil {
			return nil, httpx.FromError(err, errClassroomServiceFailed)
		}
		if !approved {
			return nil, ErrClassroomForbidden
		}
	}
	var filter *string
	if !isHost {
		filter = &userID
	}
	sessions, err := s.repo.GetRoomAttendance(ctx, roomID, filter)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	ids := make([]string, 0, len(sessions))
	for _, session := range sessions {
		ids = append(ids, session.UserID)
	}
	var userInfos []*identityClient.UserInfo
	if len(ids) > 0 {
		usersResp, err := s.identityClient.GetUsersInfo(ctx, &identityClient.GetUsersInfoRequest{Ids: ids})
		if err != nil {
			return nil, httpx.FromError(err, errClassroomServiceFailed)
		}
		userInfos = usersResp.Users
	}
	usersByID := dto.ToUserInfoMap(userInfos)
	responses := make([]*dto.AttendanceSessionResponse, len(sessions))
	for i, session := range sessions {
		responses[i] = &dto.AttendanceSessionResponse{ID: session.ID, User: usersByID[session.UserID], Role: session.Role, JoinedAt: session.JoinedAt, LeftAt: session.LeftAt}
	}
	return responses, nil
}
