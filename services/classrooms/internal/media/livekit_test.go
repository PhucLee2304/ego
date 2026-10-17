package media

import (
	"testing"
	"time"

	"github.com/go-jose/go-jose/v3"
	"github.com/go-jose/go-jose/v3/jwt"
	"github.com/livekit/protocol/auth"
	"github.com/livekit/protocol/livekit"
)

const (
	testAPIKey    = "devkey"
	testAPISecret = "local-development-secret-key-000001"
	testTokenTTL  = 5 * time.Minute
)

func TestRoomName(t *testing.T) {
	if got := RoomName(42); got != "classroom-room-42" {
		t.Fatalf("RoomName(42) = %q", got)
	}
}

func TestCreateJoinToken(t *testing.T) {
	client := New("http://livekit:7880", testAPIKey, testAPISecret, testTokenTTL)
	issuedAt := time.Now()
	token, err := client.CreateJoinToken("classroom-room-7", "student-1", "Student One", false)
	if err != nil {
		t.Fatal(err)
	}

	verifier, err := auth.ParseAPIToken(token)
	if err != nil {
		t.Fatal(err)
	}
	claims, grants, err := verifier.Verify(testAPISecret)
	if err != nil {
		t.Fatal(err)
	}
	if verifier.APIKey() != testAPIKey || verifier.Identity() != "student-1" {
		t.Fatalf("unexpected token issuer or identity: issuer=%q identity=%q", verifier.APIKey(), verifier.Identity())
	}
	if grants.Name != "Student One" || grants.Metadata != "PARTICIPANT" {
		t.Fatalf("unexpected participant claims: %#v", grants)
	}
	if grants.Video == nil || !grants.Video.RoomJoin || grants.Video.Room != "classroom-room-7" ||
		!grants.Video.GetCanPublish() || !grants.Video.GetCanSubscribe() || grants.Video.GetCanPublishData() {
		t.Fatalf("unexpected video grant: %#v", grants.Video)
	}
	validUntil := claims.Expiry.Time()
	if validUntil.Before(issuedAt.Add(testTokenTTL-time.Second)) || validUntil.After(time.Now().Add(testTokenTTL+time.Second)) {
		t.Fatalf("unexpected expiry: %s", validUntil)
	}
}

func TestCreateJoinTokenLimitsScreenShareToHost(t *testing.T) {
	client := New("http://livekit:7880", testAPIKey, testAPISecret, testTokenTTL)
	for _, tc := range []struct {
		identity    string
		host        bool
		screenShare bool
	}{
		{identity: "teacher", host: true, screenShare: true},
		{identity: "student-1", host: false, screenShare: false},
	} {
		token, err := client.CreateJoinToken("classroom-room-7", tc.identity, tc.identity, tc.host)
		if err != nil {
			t.Fatal(err)
		}
		verifier, err := auth.ParseAPIToken(token)
		if err != nil {
			t.Fatal(err)
		}
		_, grants, err := verifier.Verify(testAPISecret)
		if err != nil {
			t.Fatal(err)
		}
		video := grants.Video
		if !video.GetCanPublishSource(livekit.TrackSource_CAMERA) || !video.GetCanPublishSource(livekit.TrackSource_MICROPHONE) {
			t.Fatalf("%s must publish camera and microphone: %#v", tc.identity, video.CanPublishSources)
		}
		if video.GetCanPublishSource(livekit.TrackSource_SCREEN_SHARE) != tc.screenShare ||
			video.GetCanPublishSource(livekit.TrackSource_SCREEN_SHARE_AUDIO) != tc.screenShare {
			t.Fatalf("%s screen share grant = %#v, want %v", tc.identity, video.CanPublishSources, tc.screenShare)
		}
	}
}

func TestCreateJoinTokenRejectsExpiredAndTamperedTokens(t *testing.T) {
	expired, err := signedToken(jwt.Claims{
		Issuer:    testAPIKey,
		Subject:   "student-1",
		NotBefore: jwt.NewNumericDate(time.Now().Add(-20 * time.Minute)),
		Expiry:    jwt.NewNumericDate(time.Now().Add(-10 * time.Minute)),
	}, &auth.ClaimGrants{Video: &auth.VideoGrant{RoomJoin: true, Room: "classroom-room-7"}})
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := auth.ParseAPIToken(expired)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := verifier.Verify(testAPISecret); err == nil {
		t.Fatal("expected an expired token to be rejected")
	}

	client := New("http://livekit:7880", testAPIKey, testAPISecret, testTokenTTL)
	valid, err := client.CreateJoinToken("classroom-room-7", "student-1", "Student One", false)
	if err != nil {
		t.Fatal(err)
	}
	verifier, err = auth.ParseAPIToken(valid)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := verifier.Verify("different-secret"); err == nil {
		t.Fatal("expected a token verified with another secret to be rejected")
	}
}

func signedToken(claims jwt.Claims, grants *auth.ClaimGrants) (string, error) {
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.HS256, Key: []byte(testAPISecret)},
		(&jose.SignerOptions{}).WithType("JWT"),
	)
	if err != nil {
		return "", err
	}
	return jwt.Signed(signer).Claims(claims).Claims(grants).CompactSerialize()
}
