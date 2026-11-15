package repository

import (
	"context"
	"errors"
	"time"

	platformdb "ego/platform/db"
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

// LockRoomParticipantState đọc state kèm FOR UPDATE (dùng trong Transaction): thao tác đổi
// quyền LiveKit giữ khóa dòng tới khi lưu xong để quyền gửi LiveKit tính từ state mới nhất,
// không bị thao tác quản trị chạy song song ghi đè. Trả nil khi chưa có state.
func (r *repository) LockRoomParticipantState(ctx context.Context, roomID uint, userID string) (*model.RoomParticipantState, error) {
	var state model.RoomParticipantState
	result := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("room_id = ? AND user_id = ?", roomID, userID).Limit(1).Find(&state)
	if result.Error != nil || result.RowsAffected == 0 {
		return nil, result.Error
	}
	return &state, nil
}

// Cột của RoomParticipantState mà các thao tác trong phòng đổi.
const (
	ParticipantColMicrophone       = "microphone_allowed"
	ParticipantColCamera           = "camera_allowed"
	ParticipantColHandRaisedAt     = "hand_raised_at"
	ParticipantColRemovedAt        = "removed_at"
	ParticipantColRemovedReason    = "removed_reason"
	ParticipantColShareRequestedAt = "screen_share_requested_at"
	ParticipantColShareApprovedAt  = "screen_share_approved_at"
)

// UpdateRoomParticipantState chỉ ghi các cột được nêu (không Save cả dòng): state thường
// được đọc từ trước, ghi cả dòng sẽ đè thay đổi đồng thời của request khác (giáo viên tắt
// mic trong lúc học sinh xin chia sẻ màn hình, học sinh vừa giơ tay…).
func (r *repository) UpdateRoomParticipantState(ctx context.Context, state *model.RoomParticipantState, columns ...string) error {
	if len(columns) == 0 {
		return errors.New("UpdateRoomParticipantState: no columns")
	}
	return r.db.WithContext(ctx).Model(state).Select(columns).Updates(state).Error
}

// RequestRoomScreenShare tạo yêu cầu chia sẻ màn hình nếu chưa có (UPDATE có điều kiện,
// như RaiseRoomHand). Trả true khi thật sự vừa tạo.
func (r *repository) RequestRoomScreenShare(ctx context.Context, roomID uint, userID string, at time.Time) (bool, error) {
	result := r.db.WithContext(ctx).Model(&model.RoomParticipantState{}).
		Where("room_id = ? AND user_id = ? AND screen_share_requested_at IS NULL AND removed_at IS NULL", roomID, userID).
		Updates(map[string]any{ParticipantColShareRequestedAt: at, ParticipantColShareApprovedAt: nil})
	return result.RowsAffected > 0, result.Error
}

// RevertRoomScreenShareApproval hủy lượt duyệt chia sẻ màn hình [approvedAt] (LiveKit không
// cấp được quyền). UPDATE có điều kiện: lượt duyệt khác hoặc thay đổi sau đó giữ nguyên.
func (r *repository) RevertRoomScreenShareApproval(ctx context.Context, roomID uint, userID string, approvedAt time.Time) error {
	return r.db.WithContext(ctx).Model(&model.RoomParticipantState{}).
		Where("room_id = ? AND user_id = ? AND screen_share_approved_at = ?", roomID, userID, approvedAt).
		Update(ParticipantColShareApprovedAt, nil).Error
}

// RaiseRoomHand giơ tay nếu chưa giơ (UPDATE có điều kiện, không đọc-rồi-ghi): gửi lặp
// không đổi thời điểm giơ tay ban đầu. Trả true khi thật sự vừa giơ.
func (r *repository) RaiseRoomHand(ctx context.Context, roomID uint, userID string, at time.Time) (bool, error) {
	result := r.db.WithContext(ctx).Model(&model.RoomParticipantState{}).
		Where("room_id = ? AND user_id = ? AND hand_raised_at IS NULL AND removed_at IS NULL", roomID, userID).
		Update("hand_raised_at", at)
	return result.RowsAffected > 0, result.Error
}

// LowerRoomHands hạ tay trong room (chỉ [userID] nếu khác nil) và trả về user
// đã được hạ theo thứ tự giơ tay.
func (r *repository) LowerRoomHands(ctx context.Context, roomID uint, userID *string) ([]string, error) {
	lowered := []string{}
	err := platformdb.WithTx(ctx, r.db, func(tx *gorm.DB) error {
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

// OpenParticipantSession mở phiên nếu user chưa có phiên đang mở. INSERT ... ON CONFLICT
// trên idx_room_participants_open_session thay cho đếm-rồi-chèn: webhook join (hoặc LiveKit
// gửi lại) chạy cùng lúc với worker đối chiếu không còn lỗi vi phạm unique.
func (r *repository) OpenParticipantSession(ctx context.Context, roomID uint, userID string, role model.RoomParticipantRole, joinedAt time.Time) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:     []clause.Column{{Name: "room_id"}, {Name: "user_id"}},
		TargetWhere: clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: "left_at IS NULL AND deleted_at IS NULL"}}},
		DoNothing:   true,
	}).Create(&model.RoomParticipant{RoomID: roomID, UserID: userID, Role: role, JoinedAt: joinedAt}).Error
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
