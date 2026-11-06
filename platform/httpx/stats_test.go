package httpx

import (
	"testing"
	"time"
)

func TestStatsQueryRange(t *testing.T) {
	day := func(s string) time.Time {
		d, _ := time.ParseInLocation(time.DateOnly, s, StatsLocation)
		return d
	}
	tests := []struct {
		name               string
		query              StatsQuery
		from, to, prevFrom time.Time
	}{
		{"seven days", StatsQuery{From: "2026-11-01", To: "2026-11-07"}, day("2026-11-01"), day("2026-11-08"), day("2026-10-25")},
		{"single day", StatsQuery{From: "2026-11-06", To: "2026-11-06"}, day("2026-11-06"), day("2026-11-07"), day("2026-11-05")},
		{"swapped", StatsQuery{From: "2026-11-07", To: "2026-11-01"}, day("2026-11-01"), day("2026-11-08"), day("2026-10-25")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.query.Range()
			if !r.From.Equal(tt.from) || !r.To.Equal(tt.to) || !r.PrevFrom.Equal(tt.prevFrom) || !r.PrevTo.Equal(tt.from) {
				t.Fatalf("range = %+v", r)
			}
		})
	}
	t.Run("all time has an empty previous period", func(t *testing.T) {
		r := StatsQuery{}.Range()
		if !r.PrevFrom.Equal(r.PrevTo) || !r.To.After(time.Now()) {
			t.Fatalf("range = %+v", r)
		}
	})
}
