package service

import (
	"strings"
	"time"

	"ego/services/learning/config"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/model"

	"gorm.io/datatypes"
)

// integrityCfg trả các ngưỡng giám sát đang áp dụng, hiện chỉ có thời gian chờ trước khi
// thiết bị khác được chiếm phiên làm bài. Bắt đầu từ giá trị mặc định rồi ghi đè bằng config,
// để chỉnh được theo môi trường (vd rút ngắn khi test) mà không sửa code.
func (s *service) integrityCfg() integrityConfig {
	cfg := defaultIntegrityConfig
	if s.cfg == nil {
		return cfg
	}
	// Chỉ ghi đè khi config có giá trị dương; 0 hay âm không có nghĩa nên giữ mặc định.
	if s.cfg.IntegrityTakeoverDelayMs > 0 {
		cfg.TakeoverDelay = time.Duration(s.cfg.IntegrityTakeoverDelayMs) * time.Millisecond
	}
	return cfg
}

// integrityConsentVersion trả phiên bản điều khoản giám sát hiện hành, là bản học viên phải
// đồng ý mới được làm bài có giám sát. Lấy từ config để khi đổi nội dung điều khoản chỉ cần
// tăng phiên bản (cùng với FE) là mọi học viên phải đồng ý lại. Không có config thì dùng hằng
// mặc định, giá trị này phải khớp với hằng tương ứng ở FE.
func (s *service) integrityConsentVersion() string {
	if s.cfg == nil || s.cfg.IntegrityConsentVersion == "" {
		return config.DefaultIntegrityConsentVersion
	}
	return s.cfg.IntegrityConsentVersion
}

// checkIntegrityConsent kiểm học viên đã đồng ý điều khoản giám sát trước khi tạo lượt làm bài
// tập có giám sát. Gọi lúc tạo attempt, trước applyIntegrityPolicy.
func (s *service) checkIntegrityConsent(assignment *model.Assignment, consent *dto.IntegrityConsentRequest) error {
	// Lượt tự làm đề hoặc bài tập không giám sát thì không cần đồng ý gì.
	if assignment == nil || !assignment.Proctored {
		return nil
	}
	// Chưa gửi đồng ý: client phải hiện điều khoản cho học viên trước.
	if consent == nil || strings.TrimSpace(consent.Version) == "" {
		return ErrIntegrityConsentRequired
	}
	// Đồng ý một bản cũ (client chưa cập nhật, hoặc điều khoản vừa đổi) thì không tính, để học
	// viên luôn đồng ý đúng nội dung đang áp dụng.
	if strings.TrimSpace(consent.Version) != s.integrityConsentVersion() {
		return ErrIntegrityConsentOutdated
	}
	return nil
}

// applyIntegrityPolicy bật giám sát cho lượt làm bài tập mới nếu bài tập có giám sát, và chụp
// luật giám sát hiện hành vào attempt. Mọi kiểm tra sau này (đếm vi phạm, chờ chiếm phiên)
// đọc từ bản chụp này, nên giáo viên sửa assignment hay config đổi giữa chừng không làm đổi
// luật của bài đang làm. Gọi lúc tạo attempt, sau khi checkIntegrityConsent đã qua.
func (s *service) applyIntegrityPolicy(attempt *model.Attempt, assignment *model.Assignment, now time.Time) {
	// Lượt tự làm đề hoặc bài tập không giám sát: giữ nguyên attempt, không có luật nào để chụp.
	if assignment == nil || !assignment.Proctored {
		return
	}
	cfg := s.integrityCfg()
	// Chặn lần cuối giá trị < 1 (dữ liệu cũ hoặc sửa tay), vì 0 nghĩa là đình chỉ ngay lần rời
	// màn đầu tiên; DB cũng có CHECK max_violations >= 1.
	maxViolations := assignment.MaxViolations
	if maxViolations < 1 {
		maxViolations = 1
	}
	// Bản chụp gồm số vi phạm được phép, thời gian chờ trước khi thiết bị khác được chiếm phiên,
	// và phiên bản điều khoản mà học viên vừa đồng ý.
	version := s.integrityConsentVersion()
	policy := model.IntegrityPolicySnapshot{
		MaxViolations:   maxViolations,
		TakeoverDelayMs: int(cfg.TakeoverDelay / time.Millisecond),
		ConsentVersion:  version,
	}
	snapshot := datatypes.NewJSONType(policy)
	attempt.Proctored = true
	attempt.IntegrityPolicy = &snapshot
	// Lưu bằng chứng học viên đã đồng ý điều khoản: lúc nào và bản nào.
	attempt.IntegrityConsentAt = &now
	attempt.IntegrityConsentVersion = &version
}
