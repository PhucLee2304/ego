package repository

import (
	"context"
	platformdb "ego/platform/db"
	"ego/platform/httpx"
	"ego/services/classrooms/internal/dto"
	"ego/services/classrooms/internal/model"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Transaction(ctx context.Context, fn func(repo *Repository) error) error {
	return platformdb.WithTx(ctx, r.db, func(tx *gorm.DB) error {
		return fn(&Repository{db: tx})
	})
}

func (r *Repository) CreateClassroom(ctx context.Context, classroom *model.Classroom) error {
	return r.db.WithContext(ctx).Create(classroom).Error
}

func (r *Repository) GetClassrooms(ctx context.Context, query dto.ClassroomsQuery, teacherIDs []string) ([]*model.Classroom, int64, error) {
	var classrooms []*model.Classroom
	var total int64
	db := r.db.WithContext(ctx).Model(&model.Classroom{})
	search := strings.TrimSpace(query.Query)
	if search != "" {
		like := "%" + search + "%"
		if len(teacherIDs) > 0 {
			db = db.Where("name ILIKE ? OR description ILIKE ? OR teacher_id IN ?", like, like, teacherIDs)
		} else {
			db = db.Where("name ILIKE ? OR description ILIKE ?", like, like)
		}
	}
	if query.Active != nil {
		db = db.Where("active = ?", *query.Active)
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := db.Preload("Members", "approved = ?", true).
		Order("id ASC").
		Limit(int(query.Limit())).
		Offset(int(query.Offset())).
		Find(&classrooms).Error; err != nil {
		return nil, 0, err
	}

	return classrooms, total, nil
}

func (r *Repository) GetClassroomsByUser(ctx context.Context, userID string, query dto.ClassroomsQuery, teacherIDs []string) ([]*model.Classroom, int64, error) {
	var classrooms []*model.Classroom
	var total int64
	membership := r.db.Model(&model.Member{}).
		Select("1").
		Where("members.classroom_id = classrooms.id").
		Where("members.user_id = ? AND members.approved = ?", userID, true)
	db := r.db.WithContext(ctx).Model(&model.Classroom{}).
		Where("teacher_id = ? OR EXISTS (?)", userID, membership)
	search := strings.TrimSpace(query.Query)
	if search != "" {
		like := "%" + search + "%"
		if len(teacherIDs) > 0 {
			db = db.Where("name ILIKE ? OR description ILIKE ? OR teacher_id IN ?", like, like, teacherIDs)
		} else {
			db = db.Where("name ILIKE ? OR description ILIKE ?", like, like)
		}
	}
	if query.Active != nil {
		db = db.Where("active = ?", *query.Active)
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := db.Preload("Members", "approved = ?", true).
		Order("id ASC").
		Limit(int(query.Limit())).
		Offset(int(query.Offset())).
		Find(&classrooms).Error; err != nil {
		return nil, 0, err
	}

	return classrooms, total, nil
}

func (r *Repository) GetClassroomByID(ctx context.Context, id uint) (*model.Classroom, error) {
	var classroom model.Classroom
	if err := r.db.WithContext(ctx).
		Preload("Members", "approved = ?", true).
		First(&classroom, id).Error; err != nil {
		return nil, err
	}
	return &classroom, nil
}

func (r *Repository) GetClassroomByCode(ctx context.Context, code string) (*model.Classroom, error) {
	var classroom model.Classroom
	if err := r.db.WithContext(ctx).Where("code = ?", code).First(&classroom).Error; err != nil {
		return nil, err
	}
	return &classroom, nil
}

func (r *Repository) UpdateClassroom(ctx context.Context, classroom *model.Classroom) error {
	return r.db.WithContext(ctx).Save(classroom).Error
}

func (r *Repository) CountApprovedMembers(ctx context.Context, classroomID uint) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.Member{}).
		Joins("JOIN classrooms ON classrooms.id = members.classroom_id").
		Where("members.classroom_id = ? AND members.approved = ?", classroomID, true).
		Where("members.user_id <> classrooms.teacher_id").
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *Repository) UpsertMember(ctx context.Context, member *model.Member) error {
	var existing model.Member
	db := r.db.WithContext(ctx)
	err := db.Unscoped().Where("classroom_id = ? AND user_id = ?", member.ClassroomID, member.UserID).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return db.Create(member).Error
	}
	if err != nil {
		return err
	}
	if err := db.Unscoped().Model(&existing).Updates(map[string]any{
		"approved":    member.Approved,
		"approved_at": member.ApprovedAt,
		"joined_at":   member.JoinedAt,
		"deleted_at":  nil,
	}).Error; err != nil {
		return err
	}
	member.Model = existing.Model
	member.DeletedAt = gorm.DeletedAt{}
	return nil
}

func (r *Repository) GetMember(ctx context.Context, classroomID uint, userID string) (*model.Member, error) {
	var member model.Member
	if err := r.db.WithContext(ctx).
		Preload("Classroom").
		Where("classroom_id = ? AND user_id = ?", classroomID, userID).
		First(&member).Error; err != nil {
		return nil, err
	}
	return &member, nil
}

func (r *Repository) GetJoinRequests(ctx context.Context, classroomID uint) ([]*model.Member, error) {
	var members []*model.Member
	if err := r.db.WithContext(ctx).
		Model(&model.Member{}).
		Where("classroom_id = ? AND approved IS NULL", classroomID).
		Preload("Classroom").
		Order("updated_at DESC, id DESC").
		Find(&members).Error; err != nil {
		return nil, err
	}
	return members, nil
}

func (r *Repository) DeleteMember(ctx context.Context, member *model.Member) error {
	return r.db.WithContext(ctx).Delete(member).Error
}

func (r *Repository) CreateAssignments(ctx context.Context, assignments []*model.Assignment) error {
	if len(assignments) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&assignments).Error
}

func (r *Repository) GetAssignmentsByExamID(ctx context.Context, examID uint) ([]*model.Assignment, error) {
	var assignments []*model.Assignment
	if err := r.db.WithContext(ctx).
		Preload("Classroom").
		Preload("Submissions").
		Where("exam_id = ?", examID).
		Order("id ASC").
		Find(&assignments).Error; err != nil {
		return nil, err
	}
	return assignments, nil
}

func (r *Repository) AssignmentExistsByClassroomAndExam(ctx context.Context, classroomID uint, examID uint) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.Assignment{}).
		Where("classroom_id = ? AND exam_id = ?", classroomID, examID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *Repository) GetAssignmentsByClassroom(ctx context.Context, classroomID uint, pagination httpx.PaginationQuery) ([]*model.Assignment, int64, error) {
	var assignments []*model.Assignment
	var total int64
	db := r.db.WithContext(ctx).Model(&model.Assignment{}).Where("classroom_id = ?", classroomID)
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := db.
		Preload("Submissions").
		Order("opens_at DESC, id DESC").
		Limit(int(pagination.Limit())).
		Offset(int(pagination.Offset())).
		Find(&assignments).Error; err != nil {
		return nil, 0, err
	}
	return assignments, total, nil
}

func (r *Repository) GetAssignmentsForGradebook(ctx context.Context, classroomID uint, from *time.Time, to *time.Time) ([]*model.Assignment, error) {
	var assignments []*model.Assignment
	db := r.db.WithContext(ctx).Where("classroom_id = ?", classroomID)
	if from != nil {
		db = db.Where("due_at >= ?", *from)
	}
	if to != nil {
		db = db.Where("due_at < ?", *to)
	}
	err := db.Preload("Submissions").Order("due_at ASC, id ASC").Find(&assignments).Error
	return assignments, err
}

func (r *Repository) GetAssignmentIDsByClassroom(ctx context.Context, classroomID uint) ([]uint, error) {
	var ids []uint
	if err := r.db.WithContext(ctx).Model(&model.Assignment{}).Where("classroom_id = ?", classroomID).Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *Repository) CreateSchedule(ctx context.Context, schedule *model.Schedule) error {
	return r.db.WithContext(ctx).Create(schedule).Error
}

func (r *Repository) CreateRoom(ctx context.Context, room *model.Room) error {
	return r.db.WithContext(ctx).Create(room).Error
}

func (r *Repository) GetOrCreateRoomParticipantState(ctx context.Context, roomID uint, userID string) (*model.RoomParticipantState, error) {
	candidate := &model.RoomParticipantState{RoomID: roomID, UserID: userID, MicrophoneAllowed: true, CameraAllowed: true}
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(candidate).Error; err != nil {
		return nil, err
	}
	var state model.RoomParticipantState
	if err := r.db.WithContext(ctx).Where("room_id = ? AND user_id = ?", roomID, userID).First(&state).Error; err != nil {
		return nil, err
	}
	return &state, nil
}

// FindRoomParticipantState trả về nil khi participant chưa có state trong room.
func (r *Repository) FindRoomParticipantState(ctx context.Context, roomID uint, userID string) (*model.RoomParticipantState, error) {
	var state model.RoomParticipantState
	err := r.db.WithContext(ctx).Where("room_id = ? AND user_id = ?", roomID, userID).First(&state).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &state, nil
}

func (r *Repository) UpdateRoomParticipantState(ctx context.Context, state *model.RoomParticipantState) error {
	return r.db.WithContext(ctx).Save(state).Error
}

// LowerRoomHands hạ tay trong room (chỉ [userID] nếu khác nil) và trả về user
// đã được hạ theo thứ tự giơ tay.
func (r *Repository) LowerRoomHands(ctx context.Context, roomID uint, userID *string) ([]string, error) {
	lowered := []string{}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Model(&model.RoomParticipantState{}).Where("room_id = ? AND hand_raised_at IS NOT NULL", roomID)
		if userID != nil {
			query = query.Where("user_id = ?", *userID)
		}
		if err := query.Clauses(clause.Locking{Strength: "UPDATE"}).Order("hand_raised_at ASC").Pluck("user_id", &lowered).Error; err != nil {
			return err
		}
		if len(lowered) == 0 {
			return nil
		}
		return tx.Model(&model.RoomParticipantState{}).Where("room_id = ? AND user_id IN ?", roomID, lowered).Update("hand_raised_at", nil).Error
	})
	return lowered, err
}

func (r *Repository) GetRoomParticipantStates(ctx context.Context, roomID uint) ([]*model.RoomParticipantState, error) {
	var states []*model.RoomParticipantState
	err := r.db.WithContext(ctx).Where("room_id = ?", roomID).Order("user_id ASC").Find(&states).Error
	return states, err
}

func (r *Repository) DeleteRoomParticipantStates(ctx context.Context, roomID uint) error {
	return r.db.WithContext(ctx).Unscoped().Where("room_id = ?", roomID).Delete(&model.RoomParticipantState{}).Error
}

func (r *Repository) CreateRoomModerationAudit(ctx context.Context, audit *model.RoomModerationAudit) error {
	return r.db.WithContext(ctx).Create(audit).Error
}

func (r *Repository) GetRoomModerationAudits(ctx context.Context, roomID uint) ([]*model.RoomModerationAudit, error) {
	var audits []*model.RoomModerationAudit
	err := r.db.WithContext(ctx).Where("room_id = ?", roomID).Order("created_at ASC, id ASC").Find(&audits).Error
	return audits, err
}

func (r *Repository) GetRoomByID(ctx context.Context, id uint) (*model.Room, error) {
	var room model.Room
	if err := r.db.WithContext(ctx).Preload("Schedule").Preload("Schedule.Classroom").Where("id = ?", id).First(&room).Error; err != nil {
		return nil, err
	}
	return &room, nil
}

func (r *Repository) GetRoomByIDForUpdate(ctx context.Context, id uint) (*model.Room, error) {
	var room model.Room
	if err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Schedule").Preload("Schedule.Classroom").Where("id = ?", id).First(&room).Error; err != nil {
		return nil, err
	}
	return &room, nil
}

func (r *Repository) GetRoomByName(ctx context.Context, name string) (*model.Room, error) {
	var id uint
	if _, err := fmt.Sscanf(name, "classroom-room-%d", &id); err != nil {
		return nil, gorm.ErrRecordNotFound
	}
	return r.GetRoomByID(ctx, id)
}

// SetRoomHostAbsentSince chỉ ghi cột host_absent_since khi room còn LIVE.
// Webhook join/leave chạy song song với EndRoom nên không được Save cả dòng:
// bản ghi nạp trước khi room kết thúc sẽ ghi đè status ENDED thành LIVE.
func (r *Repository) SetRoomHostAbsentSince(ctx context.Context, roomID uint, absentSince *time.Time) error {
	return r.db.WithContext(ctx).Model(&model.Room{}).
		Where("id = ? AND status = ?", roomID, model.RoomStatusLive).
		Update("host_absent_since", absentSince).Error
}

func (r *Repository) UpdateRoom(ctx context.Context, room *model.Room) error {
	return r.db.WithContext(ctx).Save(room).Error
}
func (r *Repository) DeleteRoom(ctx context.Context, room *model.Room) error {
	return r.db.WithContext(ctx).Delete(room).Error
}

func (r *Repository) HasLiveRoomByClassroom(ctx context.Context, classroomID uint) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Room{}).Joins("JOIN schedules ON schedules.id = rooms.schedule_id AND schedules.deleted_at IS NULL").
		Where("schedules.classroom_id = ? AND rooms.status = ? AND rooms.deleted_at IS NULL", classroomID, model.RoomStatusLive).Count(&count).Error
	return count > 0, err
}

func (r *Repository) GetLiveRoomsByClassroom(ctx context.Context, classroomID uint) ([]*model.Room, error) {
	var rooms []*model.Room
	err := r.db.WithContext(ctx).Joins("JOIN schedules ON schedules.id = rooms.schedule_id AND schedules.deleted_at IS NULL").
		Where("schedules.classroom_id = ? AND rooms.status = ? AND rooms.deleted_at IS NULL", classroomID, model.RoomStatusLive).Find(&rooms).Error
	return rooms, err
}

func (r *Repository) GetRoomsForLifecycle(ctx context.Context, now time.Time) ([]*model.Room, error) {
	var rooms []*model.Room
	err := r.db.WithContext(ctx).Preload("Schedule").Where("status IN ?", []model.RoomStatus{model.RoomStatusScheduled, model.RoomStatusLive}).Find(&rooms).Error
	return rooms, err
}

func (r *Repository) OpenParticipantSession(ctx context.Context, roomID uint, userID string, role model.RoomParticipantRole, joinedAt time.Time) error {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.RoomParticipant{}).Where("room_id = ? AND user_id = ? AND left_at IS NULL", roomID, userID).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&model.RoomParticipant{RoomID: roomID, UserID: userID, Role: role, JoinedAt: joinedAt}).Error
}

func (r *Repository) CloseParticipantSession(ctx context.Context, roomID uint, userID string, leftAt time.Time) error {
	return r.db.WithContext(ctx).Model(&model.RoomParticipant{}).Where("room_id = ? AND user_id = ? AND left_at IS NULL", roomID, userID).Update("left_at", leftAt).Error
}

func (r *Repository) CloseAllParticipantSessions(ctx context.Context, roomID uint, leftAt time.Time) error {
	return r.db.WithContext(ctx).Model(&model.RoomParticipant{}).Where("room_id = ? AND left_at IS NULL", roomID).Update("left_at", leftAt).Error
}

func (r *Repository) GetOpenParticipantSessions(ctx context.Context, roomID uint) ([]*model.RoomParticipant, error) {
	var sessions []*model.RoomParticipant
	err := r.db.WithContext(ctx).Where("room_id = ? AND left_at IS NULL", roomID).Find(&sessions).Error
	return sessions, err
}

func (r *Repository) GetRoomAttendance(ctx context.Context, roomID uint, userID *string) ([]*model.RoomParticipant, error) {
	var sessions []*model.RoomParticipant
	query := r.db.WithContext(ctx).Where("room_id = ?", roomID)
	if userID != nil {
		query = query.Where("user_id = ?", *userID)
	}
	err := query.Order("joined_at ASC, id ASC").Find(&sessions).Error
	return sessions, err
}

func (r *Repository) GetScheduleByID(ctx context.Context, id uint) (*model.Schedule, error) {
	var schedule model.Schedule
	if err := r.db.WithContext(ctx).Preload("Room").Where("id = ?", id).First(&schedule).Error; err != nil {
		return nil, err
	}
	return &schedule, nil
}

func (r *Repository) GetSchedulesByRange(ctx context.Context, classroomID uint, from time.Time, to time.Time) ([]*model.Schedule, error) {
	var schedules []*model.Schedule
	if err := r.db.WithContext(ctx).
		Preload("Room").
		Where("classroom_id = ? AND starts_at < ? AND COALESCE(ends_at, starts_at) >= ?", classroomID, to, from).
		Order("starts_at ASC, id ASC").
		Find(&schedules).Error; err != nil {
		return nil, err
	}
	return schedules, nil
}

func (r *Repository) GetAssignmentsDueInRange(ctx context.Context, classroomID uint, from time.Time, to time.Time) ([]*model.Assignment, error) {
	var assignments []*model.Assignment
	if err := r.db.WithContext(ctx).
		Where("classroom_id = ? AND due_at >= ? AND due_at < ?", classroomID, from, to).
		Order("due_at ASC, id ASC").
		Find(&assignments).Error; err != nil {
		return nil, err
	}
	return assignments, nil
}

func (r *Repository) UpdateSchedule(ctx context.Context, schedule *model.Schedule) error {
	return r.db.WithContext(ctx).Save(schedule).Error
}

func (r *Repository) DeleteSchedule(ctx context.Context, schedule *model.Schedule) error {
	return r.db.WithContext(ctx).Delete(schedule).Error
}

func (r *Repository) GetAssignmentByClassroom(ctx context.Context, classroomID uint, assignmentID uint) (*model.Assignment, error) {
	var assignment model.Assignment
	if err := r.db.WithContext(ctx).
		Preload("Submissions").
		Where("classroom_id = ? AND id = ?", classroomID, assignmentID).
		First(&assignment).Error; err != nil {
		return nil, err
	}
	return &assignment, nil
}

func (r *Repository) GetAssignmentByClassroomForUpdate(ctx context.Context, classroomID uint, assignmentID uint) (*model.Assignment, error) {
	var assignment model.Assignment
	if err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Preload("Submissions").
		Where("classroom_id = ? AND id = ?", classroomID, assignmentID).
		First(&assignment).Error; err != nil {
		return nil, err
	}
	return &assignment, nil
}

func (r *Repository) GetAssignmentForAttemptValidationForUpdate(ctx context.Context, id uint) (*model.Assignment, error) {
	var assignment model.Assignment
	if err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Preload("Classroom").Where("id = ?", id).First(&assignment).Error; err != nil {
		return nil, err
	}
	return &assignment, nil
}

func (r *Repository) UpdateAssignment(ctx context.Context, assignment *model.Assignment) error {
	return r.db.WithContext(ctx).Save(assignment).Error
}

func (r *Repository) UpdateAssignmentsContentAndActive(ctx context.Context, examID uint, title string, description *string, active bool) error {
	return r.db.WithContext(ctx).Model(&model.Assignment{}).Where("exam_id = ?", examID).Updates(map[string]any{
		"title": title, "description": description, "active": active,
	}).Error
}

func (r *Repository) RestoreAssignmentActiveStates(ctx context.Context, states map[uint]bool) error {
	for id, active := range states {
		if err := r.db.WithContext(ctx).Model(&model.Assignment{}).Where("id = ?", id).Update("active", active).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) GetApprovedMembersByClassroom(ctx context.Context, classroomID uint) ([]*model.Member, error) {
	var members []*model.Member
	if err := r.db.WithContext(ctx).
		Where("classroom_id = ? AND approved = ?", classroomID, true).
		Order("joined_at ASC, id ASC").
		Find(&members).Error; err != nil {
		return nil, err
	}
	return members, nil
}

func (r *Repository) GetAssignmentForAttemptValidation(ctx context.Context, id uint) (*model.Assignment, error) {
	var assignment model.Assignment
	if err := r.db.WithContext(ctx).
		Preload("Classroom").
		Where("id = ?", id).
		First(&assignment).Error; err != nil {
		return nil, err
	}

	return &assignment, nil
}

func (r *Repository) IsStudentApprovedInClassroom(ctx context.Context, classroomID uint, userID string) (bool, error) {
	var count int64
	approved := true
	if err := r.db.WithContext(ctx).
		Model(&model.Member{}).
		Where("classroom_id = ? AND user_id = ? AND approved = ?", classroomID, userID, approved).
		Count(&count).Error; err != nil {
		return false, err
	}

	return count > 0, nil
}

func (r *Repository) UpsertAssignmentSubmission(ctx context.Context, submission *model.AssignmentSubmission) error {
	return r.db.WithContext(ctx).
		Where("assignment_id = ? AND student_id = ?", submission.AssignmentID, submission.StudentID).
		Assign(map[string]any{
			"attempt_id": submission.AttemptID,
		}).
		FirstOrCreate(submission).Error
}

func (r *Repository) UpsertAssignmentSubmissionResult(ctx context.Context, submission *model.AssignmentSubmission) error {
	return r.db.WithContext(ctx).
		Where("assignment_id = ? AND student_id = ?", submission.AssignmentID, submission.StudentID).
		Assign(map[string]any{
			"attempt_id": submission.AttemptID,
			"score":      submission.Score,
		}).
		FirstOrCreate(submission).Error
}

func (r *Repository) GetAssignmentSubmissionByAssignmentAndStudent(ctx context.Context, assignmentID uint, studentID string) (*model.AssignmentSubmission, error) {
	var submission model.AssignmentSubmission
	if err := r.db.WithContext(ctx).
		Where("assignment_id = ? AND student_id = ?", assignmentID, studentID).
		First(&submission).Error; err != nil {
		return nil, err
	}

	return &submission, nil
}

func (r *Repository) DeleteAssignmentSubmission(ctx context.Context, assignmentID uint, studentID string) error {
	return r.db.WithContext(ctx).
		Where("assignment_id = ? AND student_id = ?", assignmentID, studentID).
		Delete(&model.AssignmentSubmission{}).Error
}

func (r *Repository) CreateNotificationOutbox(ctx context.Context, event *model.NotificationOutbox) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "event_key"}}, DoNothing: true}).Create(event).Error
}

func (r *Repository) ClaimNotificationOutbox(ctx context.Context, now time.Time, limit int) ([]*model.NotificationOutbox, error) {
	var events []*model.NotificationOutbox
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("processed_at IS NULL AND next_attempt_at <= ?", now).
			Order("id ASC").Limit(limit).Find(&events).Error; err != nil {
			return err
		}
		if len(events) == 0 {
			return nil
		}
		ids := make([]uint, len(events))
		for i, event := range events {
			ids[i] = event.ID
		}
		return tx.Model(&model.NotificationOutbox{}).Where("id IN ?", ids).
			Updates(map[string]any{"next_attempt_at": now.Add(time.Minute)}).Error
	})
	return events, err
}

func (r *Repository) MarkNotificationOutboxProcessed(ctx context.Context, id uint, at time.Time) error {
	return r.db.WithContext(ctx).Model(&model.NotificationOutbox{}).Where("id = ?", id).
		Updates(map[string]any{"processed_at": at, "last_error": nil}).Error
}

func (r *Repository) MarkNotificationOutboxFailed(ctx context.Context, id uint, attempts int, next time.Time, message string) error {
	return r.db.WithContext(ctx).Model(&model.NotificationOutbox{}).Where("id = ?", id).
		Updates(map[string]any{"attempts": attempts, "next_attempt_at": next, "last_error": message}).Error
}

func (r *Repository) CancelNotificationOutbox(ctx context.Context, eventKeyPrefix string) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&model.NotificationOutbox{}).
		Where("(event_key = ? OR event_key LIKE ?) AND processed_at IS NULL", eventKeyPrefix, eventKeyPrefix+":%").
		Updates(map[string]any{"processed_at": now, "last_error": "cancelled"}).Error
}
