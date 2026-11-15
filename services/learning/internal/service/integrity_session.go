package service

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"time"

	"ego/services/learning/internal/model"
)

// claimResult là kết quả khi một kết nối xin quyền làm bài có giám sát (claim phiên):
//   - FIRST: chưa ai giữ quyền, kết nối này nhận phiên đầu tiên.
//   - RESUME: đúng phiên cũ kết nối lại (cùng token), làm tiếp.
//   - TAKEOVER: thiết bị/tab khác chiếm quyền sau khi phiên cũ ngắt đủ lâu và người dùng xác nhận.
//   - SESSION_IN_USE: phiên khác đang kết nối, từ chối.
//   - SESSION_RECOVERING: phiên cũ vừa ngắt, chờ nó kịp kết nối lại trước khi cho chiếm.
//   - SESSION_TAKEOVER_REQUIRED: được chiếm nhưng người dùng phải xác nhận trước.
//
// Mỗi lúc chỉ một phiên được làm bài, để không chia bài cho người khác làm cùng.
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

// decideSessionClaim quyết định kết quả claim (xem claimResult). liveOwner là connectionID chủ phiên đang sống
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

// hashSessionToken băm token phiên trước khi lưu, để lộ DB cũng không dùng được token.
func hashSessionToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// sessionTokenMatches so token với hash đã lưu bằng so sánh thời gian hằng, tránh đoán token
// qua thời gian phản hồi. Token rỗng luôn không khớp.
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
