// Package storagekey dựng tên file lưu trên storage từ tên gốc client gửi lên.
package storagekey

import (
	"crypto/rand"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

const (
	maxBaseLength = 50
	randomLength  = 6
	randomChars   = "abcdefghijklmnopqrstuvwxyz0123456789"
)

var (
	invalidChars = regexp.MustCompile(`[^a-z0-9_-]+`)
	validExt     = regexp.MustCompile(`^\.[a-z0-9]{1,10}$`)
)

// FileName chuẩn hoá tên gốc thành tên file an toàn và không trùng:
// "{unix ms}-{tên đã chuẩn hoá}-{6 ký tự ngẫu nhiên}{.ext}", ví dụ "1794041644123-anh-dai-dien-k3f9qx.jpg".
//   - chỉ lấy phần tên, bỏ mọi đường dẫn ("../a/b.png" thành "b.png");
//   - bỏ dấu tiếng Việt ("đ" thành "d"), viết thường, ký tự ngoài a-z0-9_- thành "-",
//     cắt còn tối đa 50 ký tự; rỗng sau khi chuẩn hoá thì dùng "file";
//   - đuôi file viết thường, chỉ giữ khi gồm 1–10 ký tự a-z0-9;
//   - timestamp và đoạn ngẫu nhiên bảo đảm client không tạo được key trùng với file đã có.
func FileName(original string) string {
	name := filepath.Base(strings.ReplaceAll(strings.TrimSpace(original), `\`, "/"))
	if name == "." || name == "/" || name == ".." {
		name = ""
	}
	ext := strings.ToLower(filepath.Ext(name))
	base := strings.TrimSuffix(name, filepath.Ext(name))
	if !validExt.MatchString(ext) {
		ext = ""
	}

	base = removeDiacritics(strings.ToLower(base))
	base = strings.Trim(invalidChars.ReplaceAllString(base, "-"), "-")
	if len(base) > maxBaseLength {
		base = strings.TrimRight(base[:maxBaseLength], "-")
	}
	if base == "" {
		base = "file"
	}
	return fmt.Sprintf("%d-%s-%s%s", time.Now().UnixMilli(), base, randomSuffix(), ext)
}

func removeDiacritics(s string) string {
	s = strings.ReplaceAll(s, "đ", "d")
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	out, _, err := transform.String(t, s)
	if err != nil {
		return s
	}
	return out
}

func randomSuffix() string {
	b := make([]byte, randomLength)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = randomChars[int(b[i])%len(randomChars)]
	}
	return string(b)
}
