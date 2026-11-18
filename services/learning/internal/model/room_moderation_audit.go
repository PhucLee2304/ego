package model

import "gorm.io/gorm"

// RoomModerationAudit là nhật ký các lần giáo viên can thiệp vào học viên trong phòng học trực tuyến.
//
//	Chỉ ghi thêm, không sửa; ghi cùng transaction với thay đổi trạng thái người tham gia.
//	 - ActorUserID: người thực hiện (giáo viên của lớp).
//	 - TargetUserID: học viên bị tác động.
//	 - Action: MUTE (tắt mic), DISABLE_CAMERA (tắt camera) hoặc REMOVE (mời ra khỏi phòng).
//	 - Reason: lý do giáo viên nhập khi REMOVE; nil với MUTE/DISABLE_CAMERA hoặc khi bỏ trống.
type RoomModerationAudit struct {
	gorm.Model
	RoomID uint `gorm:"not null;index"`
	Room   Room `gorm:"foreignKey:RoomID;references:ID;constraint:OnDelete:CASCADE"`

	ActorUserID  string `gorm:"type:text;not null;index"`
	TargetUserID string `gorm:"type:text;not null;index"`

	Action RoomModerationAuditAction `gorm:"type:text;not null;index"`
	Reason *string                   `gorm:"type:text"`
}

type RoomModerationAuditAction string

const (
	RoomModerationAuditActionMute          RoomModerationAuditAction = "MUTE"
	RoomModerationAuditActionDisableCamera RoomModerationAuditAction = "DISABLE_CAMERA"
	RoomModerationAuditActionRemove        RoomModerationAuditAction = "REMOVE"
)
