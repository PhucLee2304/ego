package service

import (
	"context"
	"errors"
	"sync"
	"testing"

	"ego/services/learning/internal/media"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"
)

type fakeParticipantRepository struct {
	repository.Repository
	findErr error
}

func (f *fakeParticipantRepository) FindRoomParticipantState(context.Context, uint, string) (*model.RoomParticipantState, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	return &model.RoomParticipantState{MicrophoneAllowed: true, CameraAllowed: true}, nil
}

type fakePermissionMedia struct {
	media.Client
	err error
}

func (f *fakePermissionMedia) SetPublishPermissions(context.Context, string, string, media.PublishPermissions) error {
	return f.err
}

// syncParticipantPermissions trả nguyên lỗi DB (Mute All không được coi là lỗi LiveKit rồi
// bỏ qua), còn lỗi LiveKit đã qua mediaError; chưa kết nối chỉ là lỗi khi requireConnected.
func TestSyncParticipantPermissionsErrors(t *testing.T) {
	dbErr := errors.New("db down")
	tests := []struct {
		name             string
		findErr          error
		mediaErr         error
		requireConnected bool
		want             error // nil: không lỗi
		notWant          error
	}{
		{name: "lỗi DB trả nguyên", findErr: dbErr, want: dbErr, notWant: ErrMediaUnavailable},
		{name: "lỗi LiveKit là media unavailable", mediaErr: errors.New("timeout"), want: ErrMediaUnavailable},
		{name: "chưa kết nối bỏ qua", mediaErr: media.ErrParticipantNotFound},
		{name: "chưa kết nối khi cần kết nối", mediaErr: media.ErrParticipantNotFound, requireConnected: true, want: ErrParticipantNotConnected},
		{name: "thành công"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &service{
				repo:            &fakeParticipantRepository{findErr: tt.findErr},
				mediaClient:     &fakePermissionMedia{err: tt.mediaErr},
				permissionLocks: new([64]sync.Mutex),
			}
			err := s.syncParticipantPermissions(context.Background(), &model.Room{}, "student", tt.requireConnected)
			if tt.want == nil {
				if err != nil {
					t.Fatalf("err = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
			if tt.notWant != nil && errors.Is(err, tt.notWant) {
				t.Fatalf("err = %v must not be %v", err, tt.notWant)
			}
		})
	}
}
