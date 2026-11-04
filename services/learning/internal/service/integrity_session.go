package service

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"time"

	"ego/services/learning/internal/model"
)

type claimResult string

const (
	claimFirst    claimResult = "FIRST"
	claimResume   claimResult = "RESUME"
	claimTakeover claimResult = "TAKEOVER"

	claimRejectInUse            claimResult = "SESSION_IN_USE"
	claimRejectRecovering       claimResult = "SESSION_RECOVERING"
	claimRejectTakeoverRequired claimResult = "SESSION_TAKEOVER_REQUIRED"
)

// claimDecision là kết quả của decideSessionClaim. Accepted thì caller cấp token mới,
// gắn kết nối và mở ân hạn; Epoch là thế hệ phiên sau khi nhận.
type claimDecision struct {
	Result     claimResult
	Accepted   bool
	Epoch      int
	RetryAfter time.Duration
	// Conflict: ghi SESSION_CONFLICT (WARNING) cho giáo viên.
	Conflict bool
}

// decideSessionClaim áp bảng mục 1.4. liveOwner là connectionID chủ phiên đang sống
// ("" nếu không có), connectionID là kết nối đang claim.
func decideSessionClaim(attempt *model.Attempt, liveOwner, connectionID, token string, takeover bool, now time.Time, cfg integrityConfig) claimDecision {
	if attempt.SessionTokenHash == nil || *attempt.SessionTokenHash == "" {
		return claimDecision{Result: claimFirst, Accepted: true, Epoch: attempt.SessionEpoch + 1}
	}
	if sessionTokenMatches(*attempt.SessionTokenHash, token) {
		if liveOwner == "" || liveOwner == connectionID {
			return claimDecision{Result: claimResume, Accepted: true, Epoch: attempt.SessionEpoch}
		}
		return claimDecision{Result: claimRejectInUse, Conflict: true}
	}
	if liveOwner != "" {
		return claimDecision{Result: claimRejectInUse, Conflict: true}
	}
	if attempt.SessionLastSeenAt != nil {
		if wait := attempt.SessionLastSeenAt.Add(cfg.TakeoverDelay).Sub(now); wait > 0 {
			return claimDecision{Result: claimRejectRecovering, RetryAfter: wait}
		}
	}
	if !takeover {
		return claimDecision{Result: claimRejectTakeoverRequired}
	}
	return claimDecision{Result: claimTakeover, Accepted: true, Epoch: attempt.SessionEpoch + 1}
}

func hashSessionToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func sessionTokenMatches(hash, token string) bool {
	if token == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(hash), []byte(hashSessionToken(token))) == 1
}

// newSessionToken trả token bí mật cho client và hash để lưu DB.
func newSessionToken() (token, hash string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	token = hex.EncodeToString(buf)
	return token, hashSessionToken(token), nil
}
