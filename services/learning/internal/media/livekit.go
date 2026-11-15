package media

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/livekit/protocol/auth"
	"github.com/livekit/protocol/livekit"
	lksdk "github.com/livekit/server-sdk-go/v2"
	"github.com/twitchtv/twirp"
)

// Client là các thao tác backend cần trên LiveKit (media server của phòng học trực tuyến).
// Service chỉ phụ thuộc interface này để test bằng fake. Tham số room là tên phòng trên
// LiveKit (RoomName), identity là user ID của người tham gia.
type Client interface {
	// CreateRoom tạo phòng LiveKit khi buổi học bắt đầu.
	CreateRoom(context.Context, string) error
	// DeleteRoom đóng phòng và ngắt mọi người khi buổi học kết thúc.
	DeleteRoom(context.Context, string) error
	// RemoveParticipant ngắt một người khỏi phòng (bị mời ra, bị khóa, không còn thuộc lớp).
	RemoveParticipant(context.Context, string, string) error
	// ListParticipants trả identity của mọi người đang kết nối trong phòng.
	ListParticipants(context.Context, string) ([]string, error)
	// CreateJoinToken cấp JWT để client kết nối vào phòng với quyền publish cho trước.
	CreateJoinToken(room, identity, name string, host bool, permissions PublishPermissions) (string, error)
	// ListScreenSharers trả về identity của những participant đang có screen-share track chưa bị mute.
	ListScreenSharers(context.Context, string) ([]string, error)
	// MuteTracks server-mute các track thuộc [sources] của participant. Server
	// mute có hiệu lực ngay cả khi client không hợp tác.
	MuteTracks(ctx context.Context, room, identity string, sources ...Source) error
	// SetPublishPermissions thay quyền publish của phiên đang kết nối.
	SetPublishPermissions(ctx context.Context, room, identity string, permissions PublishPermissions) error
}

// LiveKitClient cài Client bằng LiveKit server SDK: rooms gọi RoomService API để quản lý
// phòng/người tham gia, apiKey/apiSecret ký join token, tokenTTL là hạn của token.
type LiveKitClient struct {
	rooms     *lksdk.RoomServiceClient
	apiKey    string
	apiSecret string
	tokenTTL  time.Duration
}

// New tạo client từ URL server LiveKit và cặp API key/secret trong config.
func New(url, apiKey, apiSecret string, tokenTTL time.Duration) *LiveKitClient {
	return &LiveKitClient{
		rooms:     lksdk.NewRoomServiceClient(url, apiKey, apiSecret),
		apiKey:    apiKey,
		apiSecret: apiSecret,
		tokenTTL:  tokenTTL,
	}
}

// ErrParticipantNotFound cho biết participant hiện không kết nối vào phòng
// LiveKit, khác với lỗi media server không phản hồi.
var ErrParticipantNotFound = errors.New("[ERROR] The participant is not connected to the room")

// wrapParticipantError gắn ErrParticipantNotFound khi LiveKit trả NotFound, để service
// phân biệt "người đã rời" (bỏ qua được) với lỗi thật; lỗi gốc vẫn giữ trong chuỗi lỗi.
func wrapParticipantError(err error) error {
	var twirpErr twirp.Error
	if errors.As(err, &twirpErr) && twirpErr.Code() == twirp.NotFound {
		return errors.Join(ErrParticipantNotFound, err)
	}
	return err
}

// ErrRoomNotFound cho biết phòng không tồn tại trên LiveKit (đã bị dọn sau khi
// trống hoặc chưa được tạo), khác với lỗi media server không phản hồi.
var ErrRoomNotFound = errors.New("[ERROR] The room does not exist on the media server")

// wrapRoomError giống wrapParticipantError nhưng cho trường hợp phòng không tồn tại.
func wrapRoomError(err error) error {
	var twirpErr twirp.Error
	if errors.As(err, &twirpErr) && twirpErr.Code() == twirp.NotFound {
		return errors.Join(ErrRoomNotFound, err)
	}
	return err
}

// Source là nguồn media mà backend quản lý quyền publish.
type Source int

const (
	SourceCamera Source = iota
	SourceMicrophone
	// SourceScreenShare gồm cả video và audio của phần chia sẻ màn hình.
	SourceScreenShare
)

// matches cho biết track source của LiveKit có thuộc nguồn s không.
func (s Source) matches(source livekit.TrackSource) bool {
	switch s {
	case SourceCamera:
		return source == livekit.TrackSource_CAMERA
	case SourceMicrophone:
		return source == livekit.TrackSource_MICROPHONE
	case SourceScreenShare:
		return source == livekit.TrackSource_SCREEN_SHARE || source == livekit.TrackSource_SCREEN_SHARE_AUDIO
	}
	return false
}

// PublishPermissions là các nguồn một participant được publish. Backend tính
// từ RoomParticipantState và để LiveKit cưỡng chế, không tin client tự kiểm tra.
type PublishPermissions struct {
	Camera      bool
	Microphone  bool
	ScreenShare bool
}

// HostPermissions là quyền của giáo viên: luôn được bật thiết bị và chia sẻ màn hình trực tiếp.
func HostPermissions() PublishPermissions {
	return PublishPermissions{Camera: true, Microphone: true, ScreenShare: true}
}

// Sources đổi quyền sang danh sách track source của LiveKit (CanPublishSources).
func (p PublishPermissions) Sources() []livekit.TrackSource {
	var sources []livekit.TrackSource
	if p.Camera {
		sources = append(sources, livekit.TrackSource_CAMERA)
	}
	if p.Microphone {
		sources = append(sources, livekit.TrackSource_MICROPHONE)
	}
	if p.ScreenShare {
		sources = append(sources, livekit.TrackSource_SCREEN_SHARE, livekit.TrackSource_SCREEN_SHARE_AUDIO)
	}
	return sources
}

// canPublish phải false khi không còn nguồn nào: LiveKit hiểu danh sách nguồn
// rỗng là được publish mọi nguồn.
func (p PublishPermissions) canPublish() bool {
	return len(p.Sources()) > 0
}

// CreateRoom tạo phòng; EmptyTimeout 10 phút: phòng trống quá lâu thì LiveKit tự đóng.
func (c *LiveKitClient) CreateRoom(ctx context.Context, name string) error {
	_, err := c.rooms.CreateRoom(ctx, &livekit.CreateRoomRequest{Name: name, EmptyTimeout: 10 * 60})
	return err
}

// DeleteRoom coi phòng không còn tồn tại là đã xóa: LiveKit tự đóng phòng trống
// sau EmptyTimeout hoặc mất phòng khi restart, khi đó buổi học vẫn phải kết thúc được.
func (c *LiveKitClient) DeleteRoom(ctx context.Context, name string) error {
	_, err := c.rooms.DeleteRoom(ctx, &livekit.DeleteRoomRequest{Room: name})
	var twirpErr twirp.Error
	if errors.As(err, &twirpErr) && twirpErr.Code() == twirp.NotFound {
		return nil
	}
	return err
}

// RemoveParticipant ngắt kết nối; người không còn trong phòng trả ErrParticipantNotFound.
func (c *LiveKitClient) RemoveParticipant(ctx context.Context, room, identity string) error {
	_, err := c.rooms.RemoveParticipant(ctx, &livekit.RoomParticipantIdentity{Room: room, Identity: identity})
	return wrapParticipantError(err)
}

// ListParticipants trả identity của người đang kết nối; phòng không tồn tại trả ErrRoomNotFound.
func (c *LiveKitClient) ListParticipants(ctx context.Context, room string) ([]string, error) {
	response, err := c.rooms.ListParticipants(ctx, &livekit.ListParticipantsRequest{Room: room})
	if err != nil {
		return nil, wrapRoomError(err)
	}
	identities := make([]string, len(response.Participants))
	for i, participant := range response.Participants {
		identities[i] = participant.Identity
	}
	return identities, nil
}

// ListScreenSharers lấy danh sách người tham gia rồi lọc ai có track SCREEN_SHARE chưa mute.
func (c *LiveKitClient) ListScreenSharers(ctx context.Context, room string) ([]string, error) {
	response, err := c.rooms.ListParticipants(ctx, &livekit.ListParticipantsRequest{Room: room})
	if err != nil {
		return nil, err
	}
	var identities []string
	for _, participant := range response.Participants {
		for _, track := range participant.Tracks {
			if track.Source == livekit.TrackSource_SCREEN_SHARE && !track.Muted {
				identities = append(identities, participant.Identity)
				break
			}
		}
	}
	return identities, nil
}

// MuteTracks lấy các track của người tham gia, mute từng track đang bật thuộc sources.
// Người không còn trong phòng trả ErrParticipantNotFound.
func (c *LiveKitClient) MuteTracks(ctx context.Context, room, identity string, sources ...Source) error {
	participant, err := c.rooms.GetParticipant(ctx, &livekit.RoomParticipantIdentity{Room: room, Identity: identity})
	if err != nil {
		return wrapParticipantError(err)
	}
	for _, track := range participant.Tracks {
		if track.Muted || !slices.ContainsFunc(sources, func(source Source) bool { return source.matches(track.Source) }) {
			continue
		}
		if _, err := c.rooms.MutePublishedTrack(ctx, &livekit.MuteRoomTrackRequest{
			Room:     room,
			Identity: identity,
			TrackSid: track.Sid,
			Muted:    true,
		}); err != nil {
			return wrapParticipantError(err)
		}
	}
	return nil
}

// SetPublishPermissions đổi quyền ngay trên phiên đang kết nối (vd duyệt chia sẻ màn hình),
// không cần client vào lại phòng.
func (c *LiveKitClient) SetPublishPermissions(ctx context.Context, room, identity string, permissions PublishPermissions) error {
	// UpdateParticipant thay thế toàn bộ permission nên phải gửi lại đủ các
	// quyền đang cấp trong join token. Data channel chỉ dùng cho reaction tạm
	// thời; backend không tin data packet cho thao tác quản trị.
	_, err := c.rooms.UpdateParticipant(ctx, &livekit.UpdateParticipantRequest{
		Room:     room,
		Identity: identity,
		Permission: &livekit.ParticipantPermission{
			CanSubscribe:      true,
			CanPublish:        permissions.canPublish(),
			CanPublishData:    true,
			CanPublishSources: permissions.Sources(),
		},
	})
	return wrapParticipantError(err)
}

// CreateJoinToken cấp quyền theo state hiện tại để học sinh bị tắt mic/camera
// không lấy lại quyền bằng cách refresh hoặc vào lại phòng. Metadata của token là
// "HOST" hoặc "PARTICIPANT" để client biết vai trò; token hết hạn sau tokenTTL.
func (c *LiveKitClient) CreateJoinToken(room, identity, name string, host bool, permissions PublishPermissions) (string, error) {
	grant := &auth.VideoGrant{
		RoomJoin:       true,
		Room:           room,
		CanPublish:     boolPtr(permissions.canPublish()),
		CanSubscribe:   boolPtr(true),
		CanPublishData: boolPtr(true),
	}
	grant.SetCanPublishSources(permissions.Sources())
	return auth.NewAccessToken(c.apiKey, c.apiSecret).
		SetIdentity(identity).
		SetName(name).
		SetValidFor(c.tokenTTL).
		SetVideoGrant(grant).
		SetMetadata(map[bool]string{true: "HOST", false: "PARTICIPANT"}[host]).
		ToJWT()
}

// boolPtr vì VideoGrant dùng *bool để phân biệt false với không đặt.
func boolPtr(value bool) *bool {
	return &value
}

// RoomName là tên phòng trên LiveKit ứng với Room trong DB ("classroom-room-<id>").
func RoomName(roomID uint) string {
	return "classroom-room-" + fmt.Sprint(roomID)
}
