package service

import (
	"context"
	"errors"
	"time"

	"ego/platform/logger"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/media"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"
	learningSocket "ego/services/learning/internal/socket"
)

func (s *service) finishRoom(ctx context.Context, room *model.Room) error {
	now := time.Now()
	if err := s.repo.Transaction(ctx, func(repo repository.Repository) error {
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
	payload := learningSocket.NewRoomEvent(eventType, room.ID, dto.ToRoomResponse(room, false))
	s.roomPublisher(room.Schedule.Classroom.TeacherID, payload)
	for _, member := range members {
		s.roomPublisher(member.UserID, payload)
	}
}
