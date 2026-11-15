package repository

import (
	"context"
	"fmt"
	"time"

	"ego/services/learning/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *repository) CreateRoom(ctx context.Context, room *model.Room) error {
	return r.db.WithContext(ctx).Create(room).Error
}

func (r *repository) GetRoomByID(ctx context.Context, id uint) (*model.Room, error) {
	var room model.Room
	if err := r.db.WithContext(ctx).Preload("Schedule").Preload("Schedule.Classroom").Where("id = ?", id).First(&room).Error; err != nil {
		return nil, err
	}
	return &room, nil
}

func (r *repository) GetRoomByIDForUpdate(ctx context.Context, id uint) (*model.Room, error) {
	var room model.Room
	if err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Schedule").Preload("Schedule.Classroom").Where("id = ?", id).First(&room).Error; err != nil {
		return nil, err
	}
	return &room, nil
}

func (r *repository) GetRoomByName(ctx context.Context, name string) (*model.Room, error) {
	var id uint
	if _, err := fmt.Sscanf(name, "classroom-room-%d", &id); err != nil {
		return nil, gorm.ErrRecordNotFound
	}
	return r.GetRoomByID(ctx, id)
}

// SetRoomHostAbsentSince chỉ ghi cột host_absent_since khi room còn LIVE.
// Webhook join/leave chạy song song với EndRoom nên không được Save cả dòng:
// bản ghi nạp trước khi room kết thúc sẽ ghi đè status ENDED thành LIVE.
// Xóa (absentSince nil) chỉ ghi khi đang có giá trị, để worker gọi mỗi lượt không ghi thừa.
func (r *repository) SetRoomHostAbsentSince(ctx context.Context, roomID uint, absentSince *time.Time) error {
	query := r.db.WithContext(ctx).Model(&model.Room{}).Where("id = ? AND status = ?", roomID, model.RoomStatusLive)
	if absentSince == nil {
		query = query.Where("host_absent_since IS NOT NULL")
	}
	return query.Update("host_absent_since", absentSince).Error
}

// MarkRoomHostAbsent đặt host_absent_since chỉ khi chưa có (giữ thời điểm vắng mặt sớm hơn
// do webhook left ghi), rồi trả giá trị đang lưu.
func (r *repository) MarkRoomHostAbsent(ctx context.Context, roomID uint, at time.Time) (*time.Time, error) {
	if err := r.db.WithContext(ctx).Model(&model.Room{}).
		Where("id = ? AND status = ? AND host_absent_since IS NULL", roomID, model.RoomStatusLive).
		Update("host_absent_since", at).Error; err != nil {
		return nil, err
	}
	var room model.Room
	if err := r.db.WithContext(ctx).Select("id", "host_absent_since").Where("id = ?", roomID).Limit(1).Find(&room).Error; err != nil {
		return nil, err
	}
	return room.HostAbsentSince, nil
}

// ClearRoomHostAbsent xóa thời điểm vắng mặt ghi trước [before] (lúc lấy danh sách người
// đang kết nối): giáo viên có trong danh sách đó nhưng rời phòng ngay sau thì thời điểm
// webhook left ghi (sau [before]) được giữ.
func (r *repository) ClearRoomHostAbsent(ctx context.Context, roomID uint, before time.Time) error {
	return r.db.WithContext(ctx).Model(&model.Room{}).
		Where("id = ? AND status = ? AND host_absent_since < ?", roomID, model.RoomStatusLive, before).
		Update("host_absent_since", nil).Error
}

func (r *repository) UpdateRoom(ctx context.Context, room *model.Room) error {
	return r.db.WithContext(ctx).Save(room).Error
}
func (r *repository) DeleteRoom(ctx context.Context, room *model.Room) error {
	return r.db.WithContext(ctx).Delete(room).Error
}

func (r *repository) HasLiveRoomByClassroom(ctx context.Context, classroomID uint) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Room{}).Joins("JOIN schedules ON schedules.id = rooms.schedule_id AND schedules.deleted_at IS NULL").
		Where("schedules.classroom_id = ? AND rooms.status = ? AND rooms.deleted_at IS NULL", classroomID, model.RoomStatusLive).Count(&count).Error
	return count > 0, err
}

func (r *repository) GetLiveRoomsByClassroom(ctx context.Context, classroomID uint) ([]*model.Room, error) {
	var rooms []*model.Room
	err := r.db.WithContext(ctx).Joins("JOIN schedules ON schedules.id = rooms.schedule_id AND schedules.deleted_at IS NULL").
		Where("schedules.classroom_id = ? AND rooms.status = ? AND rooms.deleted_at IS NULL", classroomID, model.RoomStatusLive).Find(&rooms).Error
	return rooms, err
}

// GetLiveRoomsByUser trả các phòng LIVE thuộc lớp mà user là giáo viên hoặc học sinh đã được duyệt,
// kèm Schedule.Classroom để biết user có phải giáo viên của phòng không.
func (r *repository) GetLiveRoomsByUser(ctx context.Context, userID string) ([]*model.Room, error) {
	var rooms []*model.Room
	err := r.db.WithContext(ctx).Preload("Schedule.Classroom").
		Joins("JOIN schedules ON schedules.id = rooms.schedule_id AND schedules.deleted_at IS NULL").
		Joins("JOIN classrooms ON classrooms.id = schedules.classroom_id AND classrooms.deleted_at IS NULL").
		Where("rooms.status = ? AND rooms.deleted_at IS NULL", model.RoomStatusLive).
		Where(`classrooms.teacher_id = ? OR EXISTS (
			SELECT 1 FROM members WHERE members.classroom_id = classrooms.id AND members.user_id = ?
			AND members.approved = TRUE AND members.deleted_at IS NULL)`, userID, userID).
		Find(&rooms).Error
	return rooms, err
}

func (r *repository) GetRoomsForLifecycle(ctx context.Context, now time.Time) ([]*model.Room, error) {
	var rooms []*model.Room
	err := r.db.WithContext(ctx).Preload("Schedule").Preload("Schedule.Classroom").Where("status IN ?", []model.RoomStatus{model.RoomStatusScheduled, model.RoomStatusLive}).Find(&rooms).Error
	return rooms, err
}
