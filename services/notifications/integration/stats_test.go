//go:build integration

package integration_test

import (
	"context"
	"testing"
	"time"

	"ego/platform/httpx"
	"ego/services/notifications/internal/service"
)

// GetAdminStats gom data.type thành nhóm; type lạ (hoặc trống) vào OTHER.
func TestGetAdminStats(t *testing.T) {
	ctx := context.Background()
	repo, db := testRepository(t)
	createNotifications(t, repo)
	if err := db.Exec("UPDATE notifications SET read_at = now() WHERE idempotency_key = 'n-3'").Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().In(httpx.StatsLocation)
	resp, httpErr := service.New(repo, nil, nil).GetAdminStats(ctx, httpx.StatsQuery{From: now.Format(time.DateOnly), To: now.Format(time.DateOnly)})
	if httpErr != nil {
		t.Fatalf("%v: %v", httpErr, httpErr.Unwrap())
	}
	if resp.Sent.Current != 3 || resp.Read.Current != 1 || len(resp.Daily) != 1 || resp.Daily[0].Sent != 3 {
		t.Fatalf("totals = %+v daily=%+v", resp, resp.Daily)
	}
	got := map[string][2]int64{}
	for _, c := range resp.Categories {
		got[c.Category] = [2]int64{c.Sent, c.Read}
	}
	want := map[string][2]int64{"ROOMS": {1, 1}, "CLASSROOMS": {1, 0}, "OTHER": {1, 0}}
	if len(got) != len(want) {
		t.Fatalf("categories = %v", got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("categories = %v", got)
		}
	}
}
