package service

import "testing"

func TestIntegrityRegistryRestoreOwner(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(r *integrityRegistry)
		release   string // kết nối ngắt trong lúc tx claim đang chạy
		wantOwner string
		wantOld   string // chủ của attempt 1 sau khi hoàn tác claim attempt 2 ("" nếu không xét)
	}{
		{
			name:      "trả lại chủ cũ còn kết nối",
			setup:     func(r *integrityRegistry) { r.setOwner(2, "a") },
			wantOwner: "a",
		},
		{
			name:      "không trả lại chủ cũ đã ngắt kết nối",
			setup:     func(r *integrityRegistry) { r.setOwner(2, "a") },
			release:   "a",
			wantOwner: "",
		},
		{
			name:      "claim lại cùng kết nối thì giữ nguyên",
			setup:     func(r *integrityRegistry) { r.setOwner(2, "b") },
			wantOwner: "b",
		},
		{
			name:      "trả lại attempt cũ của kết nối claim",
			setup:     func(r *integrityRegistry) { r.setOwner(1, "b") },
			wantOwner: "",
			wantOld:   "b",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newIntegrityRegistry()
			tt.setup(r)
			change := r.setOwner(2, "b")
			if tt.release != "" {
				r.releaseConnection(tt.release)
			}
			r.restoreOwner(2, "b", change)
			if got := r.owner(2); got != tt.wantOwner {
				t.Fatalf("owner(2) = %q, want %q", got, tt.wantOwner)
			}
			if tt.wantOld != "" {
				if got := r.owner(1); got != tt.wantOld {
					t.Fatalf("owner(1) = %q, want %q", got, tt.wantOld)
				}
			}
		})
	}
}
