package service

import "sync"

// integrityRegistry ghi kết nối WebSocket nào đang làm chủ (đang giữ quyền làm) từng attempt có
// giám sát, để từ chối thao tác từ kết nối khác và biết chủ phiên còn kết nối hay không.
// Đây là trạng thái "đang sống" trong bộ nhớ, bổ sung cho token và epoch phiên lưu ở DB.
// learning chạy một instance nên map trong tiến trình là đủ; restart thì rỗng, client kết nối
// lại và claim lại phiên.
// Giữ hai chiều để tra nhanh cả hai hướng: owners (attempt → kết nối chủ) và conns (kết nối →
// attempt nó claim gần nhất). Một kết nối chỉ làm chủ tối đa một attempt.
type integrityRegistry struct {
	mu     sync.Mutex
	owners map[uint]string // attemptID → connectionID
	conns  map[string]uint // connectionID → attemptID
}

// newIntegrityRegistry tạo bộ nhớ trong tiến trình ghi kết nối WebSocket nào đang giữ attempt
// nào; learning chạy một instance nên không cần lưu ngoài.
func newIntegrityRegistry() *integrityRegistry {
	return &integrityRegistry{owners: make(map[uint]string), conns: make(map[string]uint)}
}

// owner trả connection ID đang giữ attempt; rỗng là chưa ai giữ.
func (r *integrityRegistry) owner(attemptID uint) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.owners[attemptID]
}

// ownerChange là những gì setOwner đã đổi, để restoreOwner hoàn tác.
type ownerChange struct {
	previousOwner   string // chủ cũ của attempt ("" nếu không có)
	previousAttempt uint   // attempt khác mà connectionID đang làm chủ (0 nếu không có)
}

// setOwner gắn connectionID làm chủ phiên của attempt. Chủ cũ vẫn giữ mục trong conns
// (releaseConnection đã so owners) để biết nó còn kết nối khi cần hoàn tác.
func (r *integrityRegistry) setOwner(attemptID uint, connectionID string) ownerChange {
	r.mu.Lock()
	defer r.mu.Unlock()
	change := ownerChange{previousOwner: r.owners[attemptID]}
	// Đã là chủ rồi (claim lặp, resume): không đổi gì.
	if change.previousOwner == connectionID {
		return change
	}
	// Kết nối này đang làm chủ attempt khác: nhả attempt đó, vì một kết nối chỉ làm chủ một
	// attempt. Ghi lại để restoreOwner trả lại được nếu claim này không commit.
	if previousAttempt, ok := r.conns[connectionID]; ok && r.owners[previousAttempt] == connectionID {
		delete(r.owners, previousAttempt)
		change.previousAttempt = previousAttempt
	}
	r.owners[attemptID] = connectionID
	r.conns[connectionID] = attemptID
	return change
}

// restoreOwner hoàn tác setOwner(attemptID, connectionID) khi transaction claim không
// commit, nếu chưa có claim nào khác ghi đè. Chủ cũ chỉ được trả lại khi kết nối của nó
// còn sống (releaseConnection chưa gỡ).
func (r *integrityRegistry) restoreOwner(attemptID uint, connectionID string, change ownerChange) {
	r.mu.Lock()
	defer r.mu.Unlock()
	// setOwner không đổi gì, hoặc đã có claim khác ghi đè sau đó: không hoàn tác, để không gỡ
	// mất chủ phiên mới hợp lệ.
	if change.previousOwner == connectionID || r.owners[attemptID] != connectionID {
		return
	}
	delete(r.owners, attemptID)
	delete(r.conns, connectionID)
	// Trả lại chủ cũ của attempt, nhưng chỉ khi kết nối đó vẫn còn và vẫn gắn với attempt này;
	// kết nối đã ngắt thì để trống cho claim sau.
	if previous := change.previousOwner; previous != "" {
		if attempt, ok := r.conns[previous]; ok && attempt == attemptID {
			r.owners[attemptID] = previous
		}
	}
	// Trả lại attempt mà kết nối này đã nhả, nếu chưa ai khác chiếm nó trong lúc đó.
	if change.previousAttempt != 0 {
		if _, taken := r.owners[change.previousAttempt]; !taken {
			r.owners[change.previousAttempt] = connectionID
			r.conns[connectionID] = change.previousAttempt
		}
	}
}

// releaseConnection gỡ một kết nối WebSocket vừa đóng. Trả attempt mà nó đang làm chủ (nếu
// có), để caller ghi lúc phiên mất kết nối và bắt đầu tính thời gian chờ trước khi cho thiết
// bị khác chiếm.
func (r *integrityRegistry) releaseConnection(connectionID string) (uint, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	attemptID, ok := r.conns[connectionID]
	if !ok {
		return 0, false
	}
	delete(r.conns, connectionID)
	// Kết nối từng claim attempt này nhưng đã bị kết nối khác chiếm: không còn là chủ, không có
	// gì để báo.
	if r.owners[attemptID] != connectionID {
		return 0, false
	}
	delete(r.owners, attemptID)
	return attemptID, true
}

// clearAttempt gỡ chủ phiên của attempt (bài đã chốt hoặc giáo viên giải phóng phiên).
func (r *integrityRegistry) clearAttempt(attemptID uint) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if connectionID, ok := r.owners[attemptID]; ok {
		delete(r.conns, connectionID)
		delete(r.owners, attemptID)
	}
}
