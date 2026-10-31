package service

import (
	"context"
	"fmt"
	"time"

	"ego/platform/httpx"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"
)

func (s *service) SetAssignmentActive(ctx context.Context, teacherID string, classroomID uint, assignmentID uint, active bool) (*dto.AssignmentResponse, *httpx.HTTPError) {
	var response *dto.AssignmentResponse
	var events []pendingEvent

	// Tắt assignment và nộp các bài đang làm trong cùng một transaction (thay RPC
	// SubmitAssignmentAttempts + bật lại assignment khi RPC lỗi).
	if err := s.repo.Transaction(ctx, func(repo repository.Repository) error {
		classroom, err := repo.GetClassroomByID(ctx, classroomID)
		if err != nil {
			return err
		}
		if classroom.TeacherID != teacherID {
			return ErrClassroomForbidden
		}
		if active && !classroom.Active {
			return ErrClassroomNotActive
		}

		// Lấy Assignment kèm khóa ghi để tránh hai request bật/tắt cập nhật đè lên nhau.
		assignment, err := repo.GetAssignmentByClassroomForUpdate(ctx, classroomID, assignmentID)
		if err != nil {
			return err
		}
		if active && !assignment.DueAt.After(time.Now()) {
			return ErrAssignmentExpired
		}

		assignment.Active = active
		if err := repo.UpdateAssignment(ctx, assignment); err != nil {
			return err
		}
		if !active {
			if err := repo.CancelNotificationOutbox(ctx, fmt.Sprintf("assignment:%d:due-reminder", assignment.ID)); err != nil {
				return err
			}
			// Khi tắt Assignment, nộp ngay mọi attempt đang làm bằng các đáp án đã lưu.
			if _, err := s.withRepoDeferringEvents(repo, &events).submitAssignmentAttempts(ctx, assignmentID, SubmitReasonAssignmentPaused); err != nil {
				return err
			}
		} else {
			members, err := repo.GetApprovedMembersByClassroom(ctx, assignment.ClassroomID)
			if err != nil {
				return err
			}
			recipients := make([]string, len(members))
			for i, member := range members {
				recipients[i] = member.UserID
			}
			reminderAt := assignment.DueAt.Add(-24 * time.Hour)
			if reminderAt.Before(time.Now()) {
				reminderAt = time.Now()
			}
			if err := createNotificationOutbox(ctx, repo, model.NotificationSourceAssignment, fmt.Sprintf("assignment:%d:due-reminder:%d", assignment.ID, assignment.UpdatedAt.UnixNano()), recipients,
				"Assignment due soon", assignment.Title+" is due soon", "ASSIGNMENT", assignment.ID, assignment.ClassroomID, reminderAt); err != nil {
				return err
			}
		}
		response = dto.ToAssignmentResponse(assignment, time.Now(), "", true, nil)
		return nil
	}); err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	s.publishPendingEvents(events)
	return response, nil
}

// updateAssignmentScheduleTx đổi lịch assignment và thời hạn các attempt đang làm trong
// transaction của người gọi. Rút ngắn lịch khi đã có attempt cần req.Confirm.
func updateAssignmentScheduleTx(ctx context.Context, repo repository.Repository, teacherID string, classroomID uint, assignmentID uint, req dto.UpdateAssignmentScheduleRequest) (*model.Assignment, error) {
	if !req.DueAt.After(req.OpensAt) {
		return nil, ErrAssignmentInvalidTime
	}
	if time.Duration(req.Duration)*time.Minute > req.DueAt.Sub(req.OpensAt) {
		return nil, ErrAssignmentDuration
	}
	classroom, err := repo.GetClassroomByID(ctx, classroomID)
	if err != nil {
		return nil, err
	}
	if classroom.TeacherID != teacherID {
		return nil, ErrClassroomForbidden
	}
	assignment, err := repo.GetAssignmentByClassroomForUpdate(ctx, classroomID, assignmentID)
	if err != nil {
		return nil, err
	}
	// Trạng thái attempt quyết định thay đổi lịch có cần giáo viên xác nhận hay không.
	_, totalAttempts, err := repo.CountAssignmentAttempts(ctx, assignmentID)
	if err != nil {
		return nil, err
	}
	if totalAttempts > 0 && !req.Confirm && (req.DueAt.Before(assignment.DueAt) || req.Duration < assignment.Duration) {
		return nil, ErrAssignmentScheduleConfirmation
	}

	assignment.Duration = req.Duration
	assignment.OpensAt = req.OpensAt
	assignment.DueAt = req.DueAt
	// Giám sát chỉ áp cho attempt tạo sau (snapshot, Q4).
	if req.Proctored != nil {
		assignment.Proctored = *req.Proctored
	}
	if req.MaxViolations != nil {
		assignment.MaxViolations = *req.MaxViolations
	}
	if err := repo.UpdateAssignment(ctx, assignment); err != nil {
		return nil, err
	}
	if err := repo.CancelNotificationOutbox(ctx, fmt.Sprintf("assignment:%d:due-reminder", assignment.ID)); err != nil {
		return nil, err
	}
	members, err := repo.GetApprovedMembersByClassroom(ctx, assignment.ClassroomID)
	if err != nil {
		return nil, err
	}
	recipients := make([]string, len(members))
	for i, member := range members {
		recipients[i] = member.UserID
	}
	reminderAt := assignment.DueAt.Add(-24 * time.Hour)
	if reminderAt.Before(time.Now()) {
		reminderAt = time.Now()
	}
	if assignment.Active {
		if err := createNotificationOutbox(ctx, repo, model.NotificationSourceAssignment, fmt.Sprintf("assignment:%d:due-reminder:%d", assignment.ID, assignment.UpdatedAt.UnixNano()), recipients,
			"Assignment due soon", assignment.Title+" is due soon", "ASSIGNMENT", assignment.ID, assignment.ClassroomID, reminderAt); err != nil {
			return nil, err
		}
	}

	// Đồng bộ thời lượng và hạn nộp sang các attempt đang làm.
	if _, err := repo.UpdateActiveAssignmentAttemptSchedule(ctx, assignmentID, req.Duration, req.DueAt); err != nil {
		return nil, err
	}
	return assignment, nil
}
