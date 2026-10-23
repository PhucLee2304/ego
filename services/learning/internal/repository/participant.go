package repository

import (
	"context"
	"time"

	"ego/services/learning/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *repository) GetOrCreateRoomParticipantState(ctx context.Context, roomID uint, userID string) (*model.RoomParticipantState, error) {
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
func (r *repository) FindRoomParticipantState(ctx context.Context, roomID uint, userID string) (*model.RoomParticipantState, error) {
	var state model.RoomParticipantState
	// Limit+Find thay cho First: "chưa có state" là trường hợp bình thường, không để GORM log record not found.
	result := r.db.WithContext(ctx).Where("room_id = ? AND user_id = ?", roomID, userID).Order("id").Limit(1).Find(&state)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	return &state, nil
}

func (r *repository) UpdateRoomParticipantState(ctx context.Context, state *model.RoomParticipantState) error {
	return r.db.WithContext(ctx).Save(state).Error
}

// LowerRoomHands hạ tay trong room (chỉ [userID] nếu khác nil) và trả về user
// đã được hạ theo thứ tự giơ tay.
func (r *repository) LowerRoomHands(ctx context.Context, roomID uint, userID *string) ([]string, error) {
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

func (r *repository) GetRoomParticipantStates(ctx context.Context, roomID uint) ([]*model.RoomParticipantState, error) {
	var states []*model.RoomParticipantState
	err := r.db.WithContext(ctx).Where("room_id = ?", roomID).Order("user_id ASC").Find(&states).Error
	return states, err
}

func (r *repository) DeleteRoomParticipantStates(ctx context.Context, roomID uint) error {
	return r.db.WithContext(ctx).Unscoped().Where("room_id = ?", roomID).Delete(&model.RoomParticipantState{}).Error
}

func (r *repository) CreateRoomModerationAudit(ctx context.Context, audit *model.RoomModerationAudit) error {
	return r.db.WithContext(ctx).Create(audit).Error
}

func (r *repository) GetRoomModerationAudits(ctx context.Context, roomID uint) ([]*model.RoomModerationAudit, error) {
	var audits []*model.RoomModerationAudit
	err := r.db.WithContext(ctx).Where("room_id = ?", roomID).Order("created_at ASC, id ASC").Find(&audits).Error
	return audits, err
}

func (r *repository) OpenParticipantSession(ctx context.Context, roomID uint, userID string, role model.RoomParticipantRole, joinedAt time.Time) error {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.RoomParticipant{}).Where("room_id = ? AND user_id = ? AND left_at IS NULL", roomID, userID).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&model.RoomParticipant{RoomID: roomID, UserID: userID, Role: role, JoinedAt: joinedAt}).Error
}

func (r *repository) CloseParticipantSession(ctx context.Context, roomID uint, userID string, leftAt time.Time) error {
	return r.db.WithContext(ctx).Model(&model.RoomParticipant{}).Where("room_id = ? AND user_id = ? AND left_at IS NULL", roomID, userID).Update("left_at", leftAt).Error
}

func (r *repository) CloseAllParticipantSessions(ctx context.Context, roomID uint, leftAt time.Time) error {
	return r.db.WithContext(ctx).Model(&model.RoomParticipant{}).Where("room_id = ? AND left_at IS NULL", roomID).Update("left_at", leftAt).Error
}

func (r *repository) GetOpenParticipantSessions(ctx context.Context, roomID uint) ([]*model.RoomParticipant, error) {
	var sessions []*model.RoomParticipant
	err := r.db.WithContext(ctx).Where("room_id = ? AND left_at IS NULL", roomID).Find(&sessions).Error
	return sessions, err
}

func (r *repository) GetRoomAttendance(ctx context.Context, roomID uint, userID *string) ([]*model.RoomParticipant, error) {
	var sessions []*model.RoomParticipant
	query := r.db.WithContext(ctx).Where("room_id = ?", roomID)
	if userID != nil {
		query = query.Where("user_id = ?", *userID)
	}
	err := query.Order("joined_at ASC, id ASC").Find(&sessions).Error
	return sessions, err
}
