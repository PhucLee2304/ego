package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	identityClient "ego/api/gen/go/identity"
	"ego/platform/httpx"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/media"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"

	"gorm.io/gorm"
)

func (s *service) RequestJoinClassroom(ctx context.Context, userID string, req dto.RequestJoinClassroomRequest) (*dto.ClassroomJoinRequestResponse, *httpx.HTTPError) {
	var response *dto.ClassroomJoinRequestResponse
	if err := s.repo.Transaction(ctx, func(repo repository.Repository) error {
		classroom, err := repo.GetClassroomByCode(ctx, strings.ToUpper(strings.TrimSpace(req.Code)))
		if err != nil {
			return err
		}
		if !classroom.Active {
			return ErrClassroomNotActive
		}
		if classroom.TeacherID == userID {
			return ErrTeacherCannotJoinClass
		}
		count, err := repo.CountApprovedMembers(ctx, classroom.ID)
		if err != nil {
			return err
		}
		if count >= int64(s.cfg.MaxStudentsPerClass) {
			return ErrClassroomCapacityFull
		}
		if existing, err := repo.GetMember(ctx, classroom.ID, userID); err == nil {
			if existing.Approved != nil && !*existing.Approved {
				existing.Approved = nil
				existing.ApprovedAt = nil
				existing.JoinedAt = nil
				if err := repo.UpsertMember(ctx, existing); err != nil {
					return err
				}
			}
			existing.Classroom = *classroom
			response = dto.ToClassroomJoinRequestResponse(existing)
			usersResp, err := s.identityClient.GetUsersInfo(ctx, &identityClient.GetUsersInfoRequest{Ids: []string{existing.UserID, classroom.TeacherID}})
			if err != nil {
				return err
			}
			usersByID := dto.ToUserInfoMap(usersResp.Users)
			response.User = usersByID[existing.UserID]
			response.Classroom.Teacher = usersByID[classroom.TeacherID]
			if err := createNotificationOutbox(ctx, repo, model.NotificationSourceMember,
				fmt.Sprintf("classroom:%d:join-request:%d:%d", classroom.ID, existing.ID, existing.UpdatedAt.UnixNano()),
				[]string{classroom.TeacherID}, "New join request", "A student requested to join "+classroom.Name,
				"CLASSROOM", classroom.ID, 0, time.Time{}); err != nil {
				return err
			}
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		member := &model.Member{
			ClassroomID: classroom.ID,
			UserID:      userID,
			Approved:    nil,
			ApprovedAt:  nil,
			JoinedAt:    nil,
		}
		if err := repo.UpsertMember(ctx, member); err != nil {
			return err
		}
		member.Classroom = *classroom
		response = dto.ToClassroomJoinRequestResponse(member)
		usersResp, err := s.identityClient.GetUsersInfo(ctx, &identityClient.GetUsersInfoRequest{Ids: []string{member.UserID, classroom.TeacherID}})
		if err != nil {
			return err
		}
		usersByID := dto.ToUserInfoMap(usersResp.Users)
		response.User = usersByID[member.UserID]
		response.Classroom.Teacher = usersByID[classroom.TeacherID]
		if err := createNotificationOutbox(ctx, repo, model.NotificationSourceMember,
			fmt.Sprintf("classroom:%d:join-request:%d:%d", classroom.ID, member.ID, member.UpdatedAt.UnixNano()),
			[]string{classroom.TeacherID}, "New join request", "A student requested to join "+classroom.Name,
			"CLASSROOM", classroom.ID, 0, time.Time{}); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	return response, nil
}

func (s *service) GetJoinRequests(ctx context.Context, teacherID string, classroomID uint) ([]*dto.ClassroomJoinRequestResponse, *httpx.HTTPError) {
	classroom, err := s.repo.GetClassroomByID(ctx, classroomID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	if classroom.TeacherID != teacherID {
		return nil, ErrClassroomForbidden
	}
	members, err := s.repo.GetJoinRequests(ctx, classroomID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}

	responses := make([]*dto.ClassroomJoinRequestResponse, len(members))
	userIDs := make([]string, 0, len(members)*2)
	seenUserIDs := make(map[string]struct{}, len(members)*2)
	for i, member := range members {
		responses[i] = dto.ToClassroomJoinRequestResponse(member)
		if _, ok := seenUserIDs[member.UserID]; !ok {
			seenUserIDs[member.UserID] = struct{}{}
			userIDs = append(userIDs, member.UserID)
		}
		if member.Classroom.TeacherID != "" {
			if _, ok := seenUserIDs[member.Classroom.TeacherID]; !ok {
				seenUserIDs[member.Classroom.TeacherID] = struct{}{}
				userIDs = append(userIDs, member.Classroom.TeacherID)
			}
		}
	}
	if len(userIDs) > 0 {
		usersResp, err := s.identityClient.GetUsersInfo(ctx, &identityClient.GetUsersInfoRequest{Ids: userIDs})
		if err != nil {
			return nil, httpx.FromError(err, errClassroomServiceFailed)
		}
		usersByID := dto.ToUserInfoMap(usersResp.Users)
		for i, response := range responses {
			response.User = usersByID[members[i].UserID]
			if response.Classroom != nil {
				response.Classroom.Teacher = usersByID[members[i].Classroom.TeacherID]
			}
		}
	}

	return responses, nil
}

func (s *service) ApproveMember(ctx context.Context, teacherID string, classroomID uint, userID string) (*dto.ClassroomJoinRequestResponse, *httpx.HTTPError) {
	var response *dto.ClassroomJoinRequestResponse
	if err := s.repo.Transaction(ctx, func(repo repository.Repository) error {
		classroom, err := repo.GetClassroomByID(ctx, classroomID)
		if err != nil {
			return err
		}
		if classroom.TeacherID != teacherID {
			return ErrClassroomForbidden
		}
		if !classroom.Active {
			return ErrClassroomNotActive
		}
		if classroom.TeacherID == userID {
			return ErrTeacherCannotJoinClass
		}
		member, err := repo.GetMember(ctx, classroomID, userID)
		if err != nil {
			return err
		}
		approved := true
		now := time.Now()
		member.Approved = &approved
		member.ApprovedAt = &now
		member.JoinedAt = &now
		if err := repo.UpsertMember(ctx, member); err != nil {
			return err
		}
		response = dto.ToClassroomJoinRequestResponse(member)
		usersResp, err := s.identityClient.GetUsersInfo(ctx, &identityClient.GetUsersInfoRequest{Ids: []string{member.UserID, member.Classroom.TeacherID}})
		if err != nil {
			return err
		}
		usersByID := dto.ToUserInfoMap(usersResp.Users)
		response.User = usersByID[member.UserID]
		response.Classroom.Teacher = usersByID[member.Classroom.TeacherID]
		if err := createNotificationOutbox(ctx, repo, model.NotificationSourceMember, fmt.Sprintf("classroom:%d:member:%d:approved", classroomID, member.ID),
			[]string{userID}, "Join request approved", "You are now a member of "+classroom.Name,
			"CLASSROOM", classroomID, 0, time.Time{}); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	return response, nil
}

func (s *service) RejectMember(ctx context.Context, teacherID string, classroomID uint, userID string) (*dto.ClassroomJoinRequestResponse, *httpx.HTTPError) {
	var response *dto.ClassroomJoinRequestResponse
	if err := s.repo.Transaction(ctx, func(repo repository.Repository) error {
		classroom, err := repo.GetClassroomByID(ctx, classroomID)
		if err != nil {
			return err
		}
		if classroom.TeacherID != teacherID {
			return ErrClassroomForbidden
		}
		if !classroom.Active {
			return ErrClassroomNotActive
		}
		member, err := repo.GetMember(ctx, classroomID, userID)
		if err != nil {
			return err
		}
		approved := false
		now := time.Now()
		member.Approved = &approved
		member.ApprovedAt = &now
		member.JoinedAt = nil
		if err := repo.UpsertMember(ctx, member); err != nil {
			return err
		}
		response = dto.ToClassroomJoinRequestResponse(member)
		usersResp, err := s.identityClient.GetUsersInfo(ctx, &identityClient.GetUsersInfoRequest{Ids: []string{member.UserID, member.Classroom.TeacherID}})
		if err != nil {
			return err
		}
		usersByID := dto.ToUserInfoMap(usersResp.Users)
		response.User = usersByID[member.UserID]
		response.Classroom.Teacher = usersByID[member.Classroom.TeacherID]
		if err := createNotificationOutbox(ctx, repo, model.NotificationSourceMember,
			fmt.Sprintf("classroom:%d:member:%d:rejected:%d", classroomID, member.ID, member.UpdatedAt.UnixNano()),
			[]string{userID}, "Join request declined", "Your request to join "+classroom.Name+" was declined",
			"CLASSROOM", classroomID, 0, time.Time{}); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	return response, nil
}

// RemoveMember thu hồi rồi xóa member và ghi outbox trong một transaction
// (trước đây khôi phục Approved bằng tay khi RPC exams lỗi).
func (s *service) RemoveMember(ctx context.Context, teacherID string, classroomID uint, userID string) *httpx.HTTPError {
	err := s.repo.Transaction(ctx, func(repo repository.Repository) error {
		classroom, err := repo.GetClassroomByID(ctx, classroomID)
		if err != nil {
			return err
		}
		if classroom.TeacherID != teacherID {
			return ErrClassroomForbidden
		}
		member, err := repo.GetMember(ctx, classroomID, userID)
		if err != nil {
			return err
		}
		revoked := false
		member.Approved = &revoked
		member.ApprovedAt = nil
		if err := repo.UpsertMember(ctx, member); err != nil {
			return err
		}
		// TODO(P20): force-submit bài assignment đang làm của học sinh (SubmitStudentAssignmentAttempts) trong cùng tx.
		if err := repo.DeleteMember(ctx, member); err != nil {
			return err
		}
		return createNotificationOutbox(ctx, repo, model.NotificationSourceMember,
			fmt.Sprintf("classroom:%d:member:%d:removed:%d", classroomID, member.ID, time.Now().UnixNano()),
			[]string{userID}, "Removed from classroom", "You were removed from "+classroom.Name,
			"CLASSROOM", classroomID, 0, time.Time{})
	})
	if err != nil {
		return httpx.FromError(err, errClassroomServiceFailed)
	}
	// Gỡ khỏi LiveKit sau khi commit; lỗi bỏ qua như trước vì member đã bị xóa.
	if s.mediaClient != nil {
		rooms, err := s.repo.GetLiveRoomsByClassroom(ctx, classroomID)
		if err != nil {
			return httpx.FromError(err, errClassroomServiceFailed)
		}
		for _, room := range rooms {
			_ = s.mediaClient.RemoveParticipant(ctx, media.RoomName(room.ID), userID)
		}
	}
	return nil
}
