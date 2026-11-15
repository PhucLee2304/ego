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

// finishRoom chuyển phòng sang ENDED, đóng mọi phiên điểm danh còn mở và xóa state tương tác
// (giơ tay, chia sẻ màn hình, quyền mic/camera) trong một transaction, rồi báo cho cả lớp.
// Phòng đã ENDED thì không làm gì, để giáo viên và worker cùng kết thúc không xung đột.
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

// ProcessRoomLifecycle chạy định kỳ từ worker cho các phòng cần xử lý:
//   - Phòng LIVE: đối chiếu người đang kết nối trên LiveKit với DB để tự sửa khi webhook bị
//     mất (mở/đóng phiên điểm danh, đá người không còn được ở lại, cập nhật host vắng mặt).
//   - Tự kết thúc phòng chưa mở mà đã quá giờ kết thúc; phòng LIVE quá 4 giờ kể từ giờ bắt
//     đầu, hoặc đã quá giờ kết thúc mà host vắng liên tục 10 phút.
//
// Trả số phòng đã xử lý.
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
			var listedAt time.Time // lúc lấy danh sách người đang kết nối
			for attempt := 1; attempt <= 3; attempt++ {
				listedAt = time.Now()
				identities, mediaErr = s.mediaClient.ListParticipants(ctx, media.RoomName(room.ID))
				if errors.Is(mediaErr, media.ErrRoomNotFound) {
					// LiveKit dọn phòng trống sau departure_timeout: không ai đang kết nối,
					// vẫn đối chiếu để đóng các phiên còn mở thay vì bỏ qua như lỗi.
					logger.Log.Debug().Uint("room_id", room.ID).Msg("[WORKER] Media room is closed, reconciling as empty")
					identities, mediaErr = nil, nil
					break
				}
				if mediaErr == nil {
					if attempt > 1 {
						logger.Log.Info().Uint("room_id", room.ID).Int("attempt", attempt).Msg("[WORKER] Participant reconcile recovered")
					}
					break
				}
				logger.Log.Warn().Err(mediaErr).Uint("room_id", room.ID).Int("attempt", attempt).Msg("[WORKER] Participant reconcile failed")
				if attempt < 3 {
					select {
					case <-ctx.Done():
						return processed, ctx.Err()
					case <-time.After(300 * time.Millisecond):
					}
				}
			}
			if mediaErr == nil {
				// Như webhook join (studentMayStayInRoom): học sinh bị mời ra/xóa khỏi lớp mà vẫn
				// kết nối (webhook đá ra lỗi hoặc bị mất) thì đá ra, không mở phiên điểm danh.
				// Nạp thành viên và state một lần cho cả phòng thay vì 2 query mỗi học sinh.
				members, err := s.repo.GetApprovedMembersByClassroom(ctx, room.Schedule.ClassroomID)
				if err != nil {
					return processed, err
				}
				states, err := s.repo.GetRoomParticipantStates(ctx, room.ID)
				if err != nil {
					return processed, err
				}
				mayStay := make(map[string]bool, len(members))
				for _, member := range members {
					mayStay[member.UserID] = true
				}
				for _, state := range states {
					if state.RemovedAt != nil {
						mayStay[state.UserID] = false
					}
				}
				connected := make(map[string]struct{}, len(identities))
				for _, identity := range identities {
					role := model.RoomParticipantRoleParticipant
					if identity == room.Schedule.Classroom.TeacherID {
						role = model.RoomParticipantRoleHost
					} else if !mayStay[identity] {
						if err := ignoreNotConnected(s.mediaClient.RemoveParticipant(ctx, media.RoomName(room.ID), identity)); err != nil {
							logger.Log.Warn().Err(err).Uint("room_id", room.ID).Str("user_id", identity).Msg("[WORKER] Failed to remove disallowed participant")
						}
						continue
					}
					connected[identity] = struct{}{}
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
					}
				}
				// Vắng mặt tính từ danh sách LiveKit và chỉ ghi có điều kiện (khi còn LIVE), không
				// Save cả room đã nạp từ đầu vòng lặp (EndRoom song song sẽ bị ghi đè ENDED thành
				// LIVE), cũng không ghi giá trị đã nạp: webhook left vừa ghi thời điểm vắng mặt
				// trong lượt này sẽ bị xóa mất và phòng không còn tự kết thúc.
				if _, ok := connected[room.Schedule.Classroom.TeacherID]; ok {
					room.HostAbsentSince = nil
					if err := s.repo.ClearRoomHostAbsent(ctx, room.ID, listedAt); err != nil {
						return processed, err
					}
				} else {
					// Lấy lại giá trị đang lưu: webhook left có thể đã ghi thời điểm sớm hơn now.
					if room.HostAbsentSince, err = s.repo.MarkRoomHostAbsent(ctx, room.ID, now); err != nil {
						return processed, err
					}
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
				logger.Log.Warn().Err(deleteErr).Uint("room_id", room.ID).Int("attempt", attempt).Msg("[WORKER] Automatic media room deletion failed")
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

// publishRoomState báo trạng thái phòng (bắt đầu, kết thúc) cho giáo viên và mọi học sinh của
// lớp qua WebSocket, để màn lịch và màn phòng cập nhật mà không phải tải lại.
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
