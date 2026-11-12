package storagekey

import (
	"regexp"
	"testing"
)

func TestFileName(t *testing.T) {
	tests := []struct {
		name     string
		original string
		want     string // phần tên giữa timestamp và đoạn ngẫu nhiên
		wantExt  string
	}{
		{name: "vietnamese", original: "Ảnh đại diện.JPG", want: "anh-dai-dien", wantExt: ".jpg"},
		{name: "path stripped", original: "../posts/a b.png", want: "a-b", wantExt: ".png"},
		{name: "windows path", original: `C:\Users\x\report.pdf`, want: "report", wantExt: ".pdf"},
		{name: "empty", original: "  ", want: "file", wantExt: ""},
		{name: "dot dot", original: "..", want: "file", wantExt: ""},
		{name: "only symbols", original: "@@@.png", want: "file", wantExt: ".png"},
		{name: "bad ext", original: "a.p/g", want: "g", wantExt: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FileName(tt.original)
			pattern := `^\d{13}-` + regexp.QuoteMeta(tt.want) + `-[a-z0-9]{6}` + regexp.QuoteMeta(tt.wantExt) + `$`
			if !regexp.MustCompile(pattern).MatchString(got) {
				t.Fatalf("FileName(%q) = %q, want match %s", tt.original, got, pattern)
			}
		})
	}
}

func TestFileNameUnique(t *testing.T) {
	if FileName("a.png") == FileName("a.png") {
		t.Fatal("two calls returned the same name")
	}
}
