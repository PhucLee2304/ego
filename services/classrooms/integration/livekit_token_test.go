package integration_test

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v3"
	"github.com/go-jose/go-jose/v3/jwt"
	"github.com/livekit/protocol/auth"
	lksdk "github.com/livekit/server-sdk-go/v2"
)

func TestLiveKitRejectsExpiredAndTamperedJoinTokens(t *testing.T) {
	url := os.Getenv("LIVEKIT_TEST_URL")
	apiKey := os.Getenv("LIVEKIT_TEST_API_KEY")
	apiSecret := os.Getenv("LIVEKIT_TEST_API_SECRET")
	if url == "" || apiKey == "" || apiSecret == "" {
		t.Skip("LiveKit integration test is not configured")
	}

	valid := liveKitJoinToken(t, apiKey, apiSecret, "token-test-room", "student-1", time.Minute)
	room, err := lksdk.ConnectToRoomWithToken(url, valid, &lksdk.RoomCallback{})
	if err != nil {
		t.Fatalf("valid token could not connect: %v", err)
	}
	room.Disconnect()

	expired := liveKitJoinToken(t, apiKey, apiSecret, "token-test-room", "student-1", -10*time.Minute)
	if room, err = lksdk.ConnectToRoomWithToken(url, expired, &lksdk.RoomCallback{}); err == nil {
		room.Disconnect()
		t.Fatal("expired token unexpectedly connected")
	}

	for name, mutate := range map[string]func(map[string]any){
		"identity": func(claims map[string]any) { claims["sub"] = "another-user" },
		"room": func(claims map[string]any) {
			claims["video"].(map[string]any)["room"] = "another-room"
		},
		"permission": func(claims map[string]any) {
			claims["video"].(map[string]any)["canPublish"] = false
		},
	} {
		t.Run("tampered_"+name, func(t *testing.T) {
			tampered := tamperToken(t, valid, mutate)
			joined, connectErr := lksdk.ConnectToRoomWithToken(url, tampered, &lksdk.RoomCallback{})
			if connectErr == nil {
				joined.Disconnect()
				t.Fatalf("token with an unsigned %s change unexpectedly connected", name)
			}
		})
	}
}

func liveKitJoinToken(t *testing.T, apiKey, apiSecret, room, identity string, validFor time.Duration) string {
	t.Helper()
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.HS256, Key: []byte(apiSecret)},
		(&jose.SignerOptions{}).WithType("JWT"),
	)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	claims := jwt.Claims{
		Issuer:    apiKey,
		Subject:   identity,
		NotBefore: jwt.NewNumericDate(now.Add(-time.Minute)),
		Expiry:    jwt.NewNumericDate(now.Add(validFor)),
	}
	grants := &auth.ClaimGrants{Video: &auth.VideoGrant{RoomJoin: true, Room: room}}
	token, err := jwt.Signed(signer).Claims(claims).Claims(grants).CompactSerialize()
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func tamperToken(t *testing.T, token string, mutate func(map[string]any)) string {
	t.Helper()
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("unexpected JWT format: %q", token)
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatal(err)
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		t.Fatal(err)
	}
	mutate(claims)
	payload, err = json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	parts[1] = base64.RawURLEncoding.EncodeToString(payload)
	return strings.Join(parts, ".")
}
