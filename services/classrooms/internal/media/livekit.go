package media

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/livekit/protocol/auth"
	"github.com/livekit/protocol/livekit"
	lksdk "github.com/livekit/server-sdk-go/v2"
	"github.com/twitchtv/twirp"
)

// ErrParticipantNotFound cho biết participant hiện không kết nối vào phòng
// LiveKit, khác với lỗi media server không phản hồi.
var ErrParticipantNotFound = errors.New("[ERROR] The participant is not connected to the room")

func wrapParticipantError(err error) error {
	var twirpErr twirp.Error
	if errors.As(err, &twirpErr) && twirpErr.Code() == twirp.NotFound {
		return errors.Join(ErrParticipantNotFound, err)
	}
	return err
}

type Client interface {
	CreateRoom(context.Context, string) error
	DeleteRoom(context.Context, string) error
	RemoveParticipant(context.Context, string, string) error
	ListParticipants(context.Context, string) ([]string, error)
	CreateJoinToken(string, string, string, bool) (string, error)
	// ListScreenSharers trả về identity của những participant đang có
	// screen-share track chưa bị mute.
	ListScreenSharers(context.Context, string) ([]string, error)
	// StopScreenShare mute mọi screen-share track của participant. Server
	// mute có hiệu lực ngay cả khi client không hợp tác tự gỡ track.
	StopScreenShare(context.Context, string, string) error
	// SetScreenSharePermission cấp hoặc thu hồi quyền publish nguồn
	// screen share cho phiên đang kết nối của participant.
	SetScreenSharePermission(context.Context, string, string, bool) error
}

var baseSources = []livekit.TrackSource{livekit.TrackSource_CAMERA, livekit.TrackSource_MICROPHONE}

var screenShareSources = []livekit.TrackSource{livekit.TrackSource_SCREEN_SHARE, livekit.TrackSource_SCREEN_SHARE_AUDIO}

// PublishSources là danh sách nguồn được publish. Giáo viên chia sẻ màn hình
// trực tiếp; học sinh chỉ có screen share khi được duyệt qua UpdateParticipant.
func PublishSources(screenShare bool) []livekit.TrackSource {
	sources := append([]livekit.TrackSource{}, baseSources...)
	if screenShare {
		sources = append(sources, screenShareSources...)
	}
	return sources
}

func isScreenShareSource(source livekit.TrackSource) bool {
	return source == livekit.TrackSource_SCREEN_SHARE || source == livekit.TrackSource_SCREEN_SHARE_AUDIO
}

type LiveKitClient struct {
	rooms     *lksdk.RoomServiceClient
	apiKey    string
	apiSecret string
	tokenTTL  time.Duration
}

func New(url, apiKey, apiSecret string, tokenTTL time.Duration) *LiveKitClient {
	return &LiveKitClient{rooms: lksdk.NewRoomServiceClient(url, apiKey, apiSecret), apiKey: apiKey, apiSecret: apiSecret, tokenTTL: tokenTTL}
}

func (c *LiveKitClient) CreateRoom(ctx context.Context, name string) error {
	_, err := c.rooms.CreateRoom(ctx, &livekit.CreateRoomRequest{Name: name, EmptyTimeout: 10 * 60})
	return err
}

func (c *LiveKitClient) DeleteRoom(ctx context.Context, name string) error {
	_, err := c.rooms.DeleteRoom(ctx, &livekit.DeleteRoomRequest{Room: name})
	return err
}

func (c *LiveKitClient) RemoveParticipant(ctx context.Context, room, identity string) error {
	_, err := c.rooms.RemoveParticipant(ctx, &livekit.RoomParticipantIdentity{Room: room, Identity: identity})
	return err
}

func (c *LiveKitClient) ListParticipants(ctx context.Context, room string) ([]string, error) {
	response, err := c.rooms.ListParticipants(ctx, &livekit.ListParticipantsRequest{Room: room})
	if err != nil {
		return nil, err
	}
	identities := make([]string, len(response.Participants))
	for i, participant := range response.Participants {
		identities[i] = participant.Identity
	}
	return identities, nil
}

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

func (c *LiveKitClient) StopScreenShare(ctx context.Context, room, identity string) error {
	participant, err := c.rooms.GetParticipant(ctx, &livekit.RoomParticipantIdentity{Room: room, Identity: identity})
	if err != nil {
		return wrapParticipantError(err)
	}
	for _, track := range participant.Tracks {
		if !isScreenShareSource(track.Source) || track.Muted {
			continue
		}
		if _, err := c.rooms.MutePublishedTrack(ctx, &livekit.MuteRoomTrackRequest{Room: room, Identity: identity, TrackSid: track.Sid, Muted: true}); err != nil {
			return wrapParticipantError(err)
		}
	}
	return nil
}

func (c *LiveKitClient) SetScreenSharePermission(ctx context.Context, room, identity string, allowed bool) error {
	// UpdateParticipant thay thế toàn bộ permission nên phải gửi lại đủ các
	// quyền đang cấp trong join token. Data channel chỉ dùng cho reaction tạm
	// thời; backend không tin data packet cho thao tác quản trị.
	_, err := c.rooms.UpdateParticipant(ctx, &livekit.UpdateParticipantRequest{
		Room:     room,
		Identity: identity,
		Permission: &livekit.ParticipantPermission{
			CanSubscribe:      true,
			CanPublish:        true,
			CanPublishData:    true,
			CanPublishSources: PublishSources(allowed),
		},
	})
	return wrapParticipantError(err)
}

func (c *LiveKitClient) CreateJoinToken(room, identity, name string, host bool) (string, error) {
	grant := &auth.VideoGrant{RoomJoin: true, Room: room, CanPublish: boolPtr(true), CanSubscribe: boolPtr(true), CanPublishData: boolPtr(true)}
	grant.SetCanPublishSources(PublishSources(host))
	return auth.NewAccessToken(c.apiKey, c.apiSecret).SetIdentity(identity).SetName(name).
		SetValidFor(c.tokenTTL).SetVideoGrant(grant).SetMetadata(map[bool]string{true: "HOST", false: "PARTICIPANT"}[host]).ToJWT()
}

func boolPtr(value bool) *bool { return &value }

func RoomName(roomID uint) string { return "classroom-room-" + fmt.Sprint(roomID) }
