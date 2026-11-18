//go:build integration

package integration_test

import (
	"context"
	"errors"
	"fmt"

	"ego/services/learning/internal/media"
)

type fakeRoomMedia struct {
	created      map[string]bool
	participants map[string][]string
	removed      []string
	createCalls  int
	deleteCalls  int
	tokenCalls   int
	listFailures int
	listCalls    int
	// closedRooms giả lập phòng LiveKit đã bị dọn (ListParticipants trả not_found).
	closedRooms map[string]bool

	sharers               map[string][]string
	stopped               []string
	screenSharePermission map[string]bool
	disconnected          map[string]bool
	// permissions là quyền publish hiện tại của phiên đang kết nối, tokens là
	// quyền ghi trong join token gần nhất của mỗi identity.
	permissions map[string]media.PublishPermissions
	tokens      map[string]media.PublishPermissions
	muted       []string
	// onList chạy giữa lúc worker đối chiếu gọi ListParticipants (giả lập thao tác song song).
	onList func()
	// failPermissions giả lập LiveKit lỗi khi đổi quyền của identity.
	failPermissions map[string]bool
}

func (f *fakeRoomMedia) CreateRoom(_ context.Context, room string) error {
	f.createCalls++
	f.created[room] = true
	return nil
}

func (f *fakeRoomMedia) DeleteRoom(_ context.Context, room string) error {
	f.deleteCalls++
	delete(f.created, room)
	return nil
}

func (f *fakeRoomMedia) RemoveParticipant(_ context.Context, room, identity string) error {
	if f.disconnected[identity] {
		return media.ErrParticipantNotFound
	}
	f.removed = append(f.removed, room+":"+identity)
	return nil
}

func (f *fakeRoomMedia) ListParticipants(_ context.Context, room string) ([]string, error) {
	f.listCalls++
	if f.onList != nil {
		f.onList()
	}
	if f.listFailures > 0 {
		f.listFailures--
		return nil, errors.New("temporary media failure")
	}
	if f.closedRooms[room] {
		return nil, media.ErrRoomNotFound
	}
	return f.participants[room], nil
}

func (f *fakeRoomMedia) CreateJoinToken(room, identity, _ string, _ bool, permissions media.PublishPermissions) (string, error) {
	f.tokenCalls++
	if f.tokens == nil {
		f.tokens = map[string]media.PublishPermissions{}
	}
	f.tokens[identity] = permissions
	return room + ":" + identity, nil
}

func (f *fakeRoomMedia) ListScreenSharers(_ context.Context, room string) ([]string, error) {
	return append([]string(nil), f.sharers[room]...), nil
}

func (f *fakeRoomMedia) MuteTracks(_ context.Context, room, identity string, sources ...media.Source) error {
	if f.disconnected[identity] {
		return media.ErrParticipantNotFound
	}
	for _, source := range sources {
		f.muted = append(f.muted, fmt.Sprintf("%s:%d", identity, source))
		if source != media.SourceScreenShare {
			continue
		}
		f.stopped = append(f.stopped, identity)
		remaining := f.sharers[room][:0]
		for _, sharer := range f.sharers[room] {
			if sharer != identity {
				remaining = append(remaining, sharer)
			}
		}
		if f.sharers != nil {
			f.sharers[room] = remaining
		}
	}
	return nil
}

func (f *fakeRoomMedia) SetPublishPermissions(_ context.Context, _ string, identity string, permissions media.PublishPermissions) error {
	if f.disconnected[identity] {
		return media.ErrParticipantNotFound
	}
	if f.failPermissions[identity] {
		return errors.New("temporary media failure")
	}
	if f.screenSharePermission == nil {
		f.screenSharePermission = map[string]bool{}
	}
	if f.permissions == nil {
		f.permissions = map[string]media.PublishPermissions{}
	}
	f.screenSharePermission[identity] = permissions.ScreenShare
	f.permissions[identity] = permissions
	return nil
}
