package media

import (
	"context"
	"fmt"
	"time"

	"github.com/livekit/protocol/auth"
	"github.com/livekit/protocol/livekit"
	lksdk "github.com/livekit/server-sdk-go/v2"
)

type Client interface {
	CreateRoom(context.Context, string) error
	DeleteRoom(context.Context, string) error
	RemoveParticipant(context.Context, string, string) error
	ListParticipants(context.Context, string) ([]string, error)
	CreateJoinToken(string, string, string, bool) (string, error)
}

type LiveKitClient struct {
	rooms     *lksdk.RoomServiceClient
	apiKey    string
	apiSecret string
}

func New(url, apiKey, apiSecret string) *LiveKitClient {
	return &LiveKitClient{rooms: lksdk.NewRoomServiceClient(url, apiKey, apiSecret), apiKey: apiKey, apiSecret: apiSecret}
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

func (c *LiveKitClient) CreateJoinToken(room, identity, name string, host bool) (string, error) {
	grant := &auth.VideoGrant{RoomJoin: true, Room: room, CanPublish: boolPtr(true), CanSubscribe: boolPtr(true), CanPublishData: boolPtr(false)}
	return auth.NewAccessToken(c.apiKey, c.apiSecret).SetIdentity(identity).SetName(name).
		SetValidFor(5 * time.Minute).SetVideoGrant(grant).SetMetadata(map[bool]string{true: "HOST", false: "PARTICIPANT"}[host]).ToJWT()
}

func boolPtr(value bool) *bool { return &value }

func RoomName(roomID uint) string { return "classroom-room-" + fmt.Sprint(roomID) }
