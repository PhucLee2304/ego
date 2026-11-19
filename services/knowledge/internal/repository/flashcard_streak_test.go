package repository

import (
	"testing"
	"time"
)

func TestStreakFromReviewDaysUsesUTCPlusSevenCalendarDays(t *testing.T) {
	// 19 Nov 17:30 UTC is already 20 Nov 00:30 for the learner.
	now := time.Date(2026, 11, 19, 17, 30, 0, 0, time.UTC)
	day := func(n int) time.Time { return time.Date(2026, 11, n, 0, 0, 0, 0, time.UTC) }
	tests := []struct {
		name          string
		days          []time.Time
		streak        int
		reviewedToday bool
	}{
		{name: "reviewed today", days: []time.Time{day(20), day(19), day(18)}, streak: 3, reviewedToday: true},
		{name: "yesterday preserved until today", days: []time.Time{day(19), day(18)}, streak: 2},
		{name: "duplicate reviews count once", days: []time.Time{day(20), day(20), day(19)}, streak: 2, reviewedToday: true},
		{name: "gap breaks streak", days: []time.Time{day(20), day(18)}, streak: 1, reviewedToday: true},
		{name: "stale streak", days: []time.Time{day(18), day(17)}},
		{name: "no reviews"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			streak, reviewedToday := streakFromReviewDays(tt.days, now)
			if streak != tt.streak || reviewedToday != tt.reviewedToday {
				t.Fatalf("got (%d, %t), want (%d, %t)", streak, reviewedToday, tt.streak, tt.reviewedToday)
			}
		})
	}
}
