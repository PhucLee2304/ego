package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"ego/platform/httpx"
	"ego/platform/logger"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/media"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"

	"gorm.io/gorm"
)

// RequestJoinClassroom cho học sinh xin vào lớp bằng mã lớp. Đã là thành viên hoặc đang chờ
// duyệt thì trả trạng thái hiện tại; đã bị từ chối thì được xin lại. Lớp đủ sĩ số thì từ chối.
func (s *service) RequestJoinClassroom(ctx context.Context, userID string, req dto.RequestJoinClassroomRequest) (*dto.ClassroomJoinRequestResponse, *httpx.HTTPError) {
	var member *model.Member
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
		existing, err := repo.GetMember(ctx, classroom.ID, userID)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		// Đã là thành viên hoặc đang chờ duyệt: trả trạng thái hiện tại, không báo lại giáo viên.
		if err == nil && (existing.Approved == nil || *existing.Approved) {
			existing.Classroom = *classroom
			member = existing
			return nil
		}
		count, err := repo.CountApprovedMembers(ctx, classroom.ID)
		if err != nil {
			return err
		}
		if count >= int64(s.cfg.MaxStudentsPerClass) {
			return ErrClassroomCapacityFull
		}
		// Chưa có hoặc đã bị từ chối: tạo (lại) yêu cầu chờ duyệt.
		member = existing
		if member == nil {
			member = &model.Member{ClassroomID: classroom.ID, UserID: userID}
		}
		member.Approved = nil
		member.ApprovedAt = nil
		member.JoinedAt = nil
		if err := repo.UpsertMember(ctx, member); err != nil {
			return err
		}
		member.Classroom = *classroom
		return createNotificationOutbox(ctx, repo, model.NotificationSourceMember,
			fmt.Sprintf("classroom:%d:join-request:%d:%d", classroom.ID, member.ID, member.UpdatedAt.UnixNano()),
			[]string{classroom.TeacherID}, "New join request", "A student requested to join "+classroom.Name,
			"CLASSROOM", classroom.ID, 0, time.Time{})
	}); err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	return s.toJoinRequestResponse(ctx, member), nil
}

// toJoinRequestResponse map member và điền thông tin học sinh + giáo viên; gọi sau khi commit.
func (s *service) toJoinRequestResponse(ctx context.Context, member *model.Member) *dto.ClassroomJoinRequestResponse {
	response := dto.ToClassroomJoinRequestResponse(member)
	usersByID := s.usersInfo(ctx, member.UserID, member.Classroom.TeacherID)
	response.User = usersByID[member.UserID]
	if response.Classroom != nil {
		response.Classroom.Teacher = usersByID[member.Classroom.TeacherID]
	}
	return response
}

// GetJoinRequests liệt kê các yêu cầu vào lớp đang chờ duyệt cho giáo viên, kèm thông tin
// người xin lấy từ identity.
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
	usersByID := s.usersInfo(ctx, userIDs...)
	for i, response := range responses {
		response.User = usersByID[members[i].UserID]
		if response.Classroom != nil {
			response.Classroom.Teacher = usersByID[members[i].Classroom.TeacherID]
		}
	}

	return responses, nil
}

// ApproveMember cho giáo viên duyệt học sinh vào lớp và xếp thông báo cho học sinh trong cùng
// transaction.
func (s *service) ApproveMember(ctx context.Context, teacherID string, classroomID uint, userID string) (*dto.ClassroomJoinRequestResponse, *httpx.HTTPError) {
	var result *model.Member
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
		result = member
		if err := createNotificationOutbox(ctx, repo, model.NotificationSourceMember, fmt.Sprintf("classroom:%d:member:%d:approved", classroomID, member.ID),
			[]string{userID}, "Join request approved", "You are now a member of "+classroom.Name,
			"CLASSROOM", classroomID, 0, time.Time{}); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	return s.toJoinRequestResponse(ctx, result), nil
}

// RejectMember cho giáo viên từ chối yêu cầu vào lớp. Bản ghi được giữ (Approved = false) để
// học sinh xin lại được; key thông báo kèm thời điểm để mỗi lần từ chối đều có thông báo riêng.
func (s *service) RejectMember(ctx context.Context, teacherID string, classroomID uint, userID string) (*dto.ClassroomJoinRequestResponse, *httpx.HTTPError) {
	var result *model.Member
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
		result = member
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
	return s.toJoinRequestResponse(ctx, result), nil
}

// RemoveMember thu hồi member, nộp bài assignment đang làm, xóa member và ghi outbox
// trong một transaction (trước đây khôi phục Approved bằng tay khi RPC exams lỗi).
func (s *service) RemoveMember(ctx context.Context, teacherID string, classroomID uint, userID string) *httpx.HTTPError {
	var events []pendingEvent
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
		assignmentIDs, err := repo.GetAssignmentIDsByClassroom(ctx, classroomID)
		if err != nil {
			return err
		}
		if _, err := s.withRepoDeferringEvents(repo, &events).submitStudentAssignmentAttempts(ctx, userID, assignmentIDs); err != nil {
			return err
		}
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
	s.publishPendingEvents(events)
	// Gỡ khỏi LiveKit sau khi commit; lỗi bỏ qua như trước vì member đã bị xóa.
	if s.mediaClient != nil {
		rooms, err := s.repo.GetLiveRoomsByClassroom(ctx, classroomID)
		if err != nil {
			// Member đã bị xóa: chỉ ghi log, không báo lỗi cho thao tác đã thành công.
			logger.Log.Error().Err(err).Uint("classroom_id", classroomID).Str("user_id", userID).Msg("[ERROR] Failed to load live rooms after removing member")
			return nil
		}
		for _, room := range rooms {
			_ = s.mediaClient.RemoveParticipant(ctx, media.RoomName(room.ID), userID)
		}
	}
	return nil
}
