package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	identityClient "ego/api/gen/go/identity"
	"ego/platform/logger"
	"ego/services/classrooms/internal/dto"
	"ego/services/classrooms/internal/media"
	"ego/services/classrooms/internal/model"
	"ego/services/classrooms/internal/repository"
	classroomsSocket "ego/services/classrooms/internal/socket"
)

func (s *service) GetRoom(ctx context.Context, userID string, roomID uint) (*dto.RoomResponse, error) {
	room, err := s.repo.GetRoomByID(ctx, roomID)
	if err != nil {
		return nil, err
	}
	isHost := room.Schedule.Classroom.TeacherID == userID
	if !isHost {
		approved, err := s.repo.IsStudentApprovedInClassroom(ctx, room.Schedule.ClassroomID, userID)
		if err != nil {
			return nil, err
		}
		if !approved {
			return nil, ErrClassroomForbidden
		}
	}
	return dto.ToRoomResponse(room, isHost), nil
}

func (s *service) GetRoomActivity(ctx context.Context, userID string, classroomID uint, roomID uint) (*dto.RoomActivityResponse, error) {
	room, err := s.repo.GetRoomByID(ctx, roomID)
	if err != nil {
		return nil, err
	}
	if room.Schedule.ClassroomID != classroomID {
		return nil, ErrClassroomForbidden
	}
	isHost := room.Schedule.Classroom.TeacherID == userID
	if !isHost {
		approved, err := s.repo.IsStudentApprovedInClassroom(ctx, classroomID, userID)
		if err != nil {
			return nil, err
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
		return nil, err
	}
	return dto.ToRoomActivityResponse(room, states, userID, isHost), nil
}

func (s *service) StartRoom(ctx context.Context, teacherID string, roomID uint) (*dto.RoomResponse, error) {
	if s.mediaClient == nil {
		return nil, ErrMediaUnavailable
	}
	room, err := s.repo.GetRoomByID(ctx, roomID)
	if err != nil {
		return nil, err
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
		return nil, errors.Join(ErrMediaUnavailable, err)
	}
	now := time.Now()
	err = s.repo.Transaction(ctx, func(repo *repository.Repository) error {
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
		if err := createNotificationOutbox(ctx, repo, fmt.Sprintf("room:%d:started", roomID), recipients,
			"Class has started", locked.Schedule.Title+" is now live", "ROOM", roomID, locked.Schedule.ClassroomID, time.Time{}); err != nil {
			return err
		}
		room = locked
		return nil
	})
	if err != nil {
		_ = s.mediaClient.DeleteRoom(context.WithoutCancel(ctx), media.RoomName(roomID))
		return nil, err
	}
	s.publishRoomState(ctx, room, "ROOM_STARTED")
	return dto.ToRoomResponse(room, true), nil
}

func (s *service) JoinRoom(ctx context.Context, userID string, roomID uint) (*dto.JoinRoomResponse, error) {
	if s.mediaClient == nil {
		return nil, ErrMediaUnavailable
	}
	room, err := s.repo.GetRoomByID(ctx, roomID)
	if err != nil {
		return nil, err
	}
	isHost := room.Schedule.Classroom.TeacherID == userID
	if !isHost {
		approved, err := s.repo.IsStudentApprovedInClassroom(ctx, room.Schedule.ClassroomID, userID)
		if err != nil {
			return nil, err
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
		return nil, err
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
		return nil, errors.Join(ErrMediaUnavailable, err)
	}
	return &dto.JoinRoomResponse{Room: dto.ToRoomResponse(room, isHost), URL: s.cfg.LiveKitPublicURL, Token: token}, nil
}

func (s *service) EndRoom(ctx context.Context, teacherID string, roomID uint) (*dto.RoomResponse, error) {
	room, err := s.repo.GetRoomByID(ctx, roomID)
	if err != nil {
		return nil, err
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
		return nil, errors.Join(ErrMediaUnavailable, err)
	}
	if err := s.finishRoom(ctx, room); err != nil {
		return nil, err
	}
	return dto.ToRoomResponse(room, true), nil
}

func (s *service) finishRoom(ctx context.Context, room *model.Room) error {
	now := time.Now()
	if err := s.repo.Transaction(ctx, func(repo *repository.Repository) error {
		locked, err := repo.GetRoomByIDForUpdate(ctx, room.ID)
		if err != nil {
			return err
		}
		if locked.Status == model.RoomStatusEnded {
			*room = *locked
			return nil
		}
		locked.Status, locked.EndedAt, locked.HostAbsentSince = model.RoomStatusEnded, &now, nil
		if err := repo.UpdateRoom(ctx, locked); err != nil {
			return err
		}
		if err := repo.CloseAllParticipantSessions(ctx, locked.ID, now); err != nil {
			return err
		}
		if err := repo.DeleteRoomParticipantStates(ctx, locked.ID); err != nil {
			return err
		}
		*room = *locked
		return nil
	}); err != nil {
		return err
	}
	s.publishRoomState(ctx, room, "ROOM_ENDED")
	return nil
}

func (s *service) HandleParticipantJoined(ctx context.Context, roomName, userID string, joinedAt time.Time) error {
	room, err := s.repo.GetRoomByName(ctx, roomName)
	if err != nil {
		return err
	}
	role := model.RoomParticipantRoleParticipant
	if userID == room.Schedule.Classroom.TeacherID {
		role = model.RoomParticipantRoleHost
		if err := s.repo.SetRoomHostAbsentSince(ctx, room.ID, nil); err != nil {
			return err
		}
	}
	return s.repo.OpenParticipantSession(ctx, room.ID, userID, role, joinedAt)
}

func (s *service) HandleParticipantLeft(ctx context.Context, roomName, userID string, leftAt time.Time) error {
	room, err := s.repo.GetRoomByName(ctx, roomName)
	if err != nil {
		return err
	}
	if err := s.repo.CloseParticipantSession(ctx, room.ID, userID, leftAt); err != nil {
		return err
	}
	// Phê duyệt chia sẻ màn hình gắn với phiên đang kết nối; vào lại phòng
	// bằng token mới thì phải xin lại.
	if changed, err := s.clearScreenShareState(ctx, room.ID, userID); err != nil {
		return err
	} else if changed {
		s.publishRoomActivity(room, classroomsSocket.EventScreenShareStopped, userID)
	}
	if userID == room.Schedule.Classroom.TeacherID {
		return s.repo.SetRoomHostAbsentSince(ctx, room.ID, &leftAt)
	}
	return nil
}

func (s *service) GetRoomAttendance(ctx context.Context, userID string, roomID uint) ([]*dto.AttendanceSessionResponse, error) {
	room, err := s.repo.GetRoomByID(ctx, roomID)
	if err != nil {
		return nil, err
	}
	isHost := room.Schedule.Classroom.TeacherID == userID
	if !isHost {
		approved, err := s.repo.IsStudentApprovedInClassroom(ctx, room.Schedule.ClassroomID, userID)
		if err != nil {
			return nil, err
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
		return nil, err
	}
	ids := make([]string, 0, len(sessions))
	for _, session := range sessions {
		ids = append(ids, session.UserID)
	}
	var userInfos []*identityClient.UserInfo
	if len(ids) > 0 {
		usersResp, err := s.identityClient.GetUsersInfo(ctx, &identityClient.GetUsersInfoRequest{Ids: ids})
		if err != nil {
			return nil, err
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

func (s *service) ProcessRoomLifecycle(ctx context.Context) (int, error) {
	rooms, err := s.repo.GetRoomsForLifecycle(ctx, time.Now())
	if err != nil {
		return 0, err
	}
	now, processed := time.Now(), 0
	for _, room := range rooms {
		// Đối chiếu trạng thái media để tự sửa dữ liệu khi webhook join/leave bị thất lạc.
		if room.Status == model.RoomStatusLive && s.mediaClient != nil {
			var identities []string
			var mediaErr error
			for attempt := 1; attempt <= 3; attempt++ {
				identities, mediaErr = s.mediaClient.ListParticipants(ctx, media.RoomName(room.ID))
				if errors.Is(mediaErr, media.ErrRoomNotFound) {
					// LiveKit dọn phòng trống sau departure_timeout: không ai đang kết nối,
					// vẫn đối chiếu để đóng các phiên còn mở thay vì bỏ qua như lỗi.
					logger.Log.Debug().Uint("room_id", room.ID).Msg("[ROOM] Media room is closed, reconciling as empty")
					identities, mediaErr = nil, nil
					break
				}
				if mediaErr == nil {
					if attempt > 1 {
						logger.Log.Info().Uint("room_id", room.ID).Int("attempt", attempt).Msg("[ROOM] Participant reconcile recovered")
					}
					break
				}
				logger.Log.Warn().Err(mediaErr).Uint("room_id", room.ID).Int("attempt", attempt).Msg("[ROOM] Participant reconcile failed")
				if attempt < 3 {
					select {
					case <-ctx.Done():
						return processed, ctx.Err()
					case <-time.After(300 * time.Millisecond):
					}
				}
			}
			if mediaErr == nil {
				connected := make(map[string]struct{}, len(identities))
				for _, identity := range identities {
					connected[identity] = struct{}{}
					role := model.RoomParticipantRoleParticipant
					if identity == room.Schedule.Classroom.TeacherID {
						role = model.RoomParticipantRoleHost
						room.HostAbsentSince = nil
					}
					if err := s.repo.OpenParticipantSession(ctx, room.ID, identity, role, now); err != nil {
						return processed, err
					}
				}
				openSessions, err := s.repo.GetOpenParticipantSessions(ctx, room.ID)
				if err != nil {
					return processed, err
				}
				for _, session := range openSessions {
					if _, ok := connected[session.UserID]; !ok {
						if err := s.repo.CloseParticipantSession(ctx, room.ID, session.UserID, now); err != nil {
							return processed, err
						}
						if session.Role == model.RoomParticipantRoleHost && room.HostAbsentSince == nil {
							room.HostAbsentSince = &now
						}
					}
				}
				if err := s.repo.UpdateRoom(ctx, room); err != nil {
					return processed, err
				}
			}
		}
		shouldEnd := room.Status == model.RoomStatusScheduled && !now.Before(room.Schedule.EndsAt)
		if room.Status == model.RoomStatusLive {
			shouldEnd = !now.Before(room.Schedule.StartsAt.Add(4*time.Hour)) || (now.After(room.Schedule.EndsAt) && room.HostAbsentSince != nil && now.Sub(*room.HostAbsentSince) >= 10*time.Minute)
		}
		if !shouldEnd {
			continue
		}
		if room.Status == model.RoomStatusLive && s.mediaClient != nil {
			var deleteErr error
			for attempt := 1; attempt <= 3; attempt++ {
				deleteErr = s.mediaClient.DeleteRoom(ctx, media.RoomName(room.ID))
				if deleteErr == nil {
					break
				}
				logger.Log.Warn().Err(deleteErr).Uint("room_id", room.ID).Int("attempt", attempt).Msg("[ROOM] Automatic media room deletion failed")
				if attempt < 3 {
					select {
					case <-ctx.Done():
						return processed, ctx.Err()
					case <-time.After(300 * time.Millisecond):
					}
				}
			}
			if deleteErr != nil {
				return processed, errors.Join(ErrMediaUnavailable, deleteErr)
			}
		}
		if err := s.finishRoom(ctx, room); err != nil {
			return processed, err
		}
		processed++
	}
	return processed, nil
}

func (s *service) publishRoomState(ctx context.Context, room *model.Room, eventType string) {
	if s.roomPublisher == nil {
		return
	}
	members, err := s.repo.GetApprovedMembersByClassroom(ctx, room.Schedule.ClassroomID)
	if err != nil {
		return
	}
	payload := classroomsSocket.NewRoomEvent(eventType, room.ID, dto.ToRoomResponse(room, false))
	s.roomPublisher(room.Schedule.Classroom.TeacherID, payload)
	for _, member := range members {
		s.roomPublisher(member.UserID, payload)
	}
}
