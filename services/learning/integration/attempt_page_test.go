//go:build integration

package integration_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"ego/platform/httpx"
	"ego/services/learning/config"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"
	"ego/services/learning/internal/service"
)

func requireStatus(t *testing.T, httpErr *httpx.HTTPError, status int) {
	t.Helper()
	if httpErr == nil || httpErr.StatusCode != status {
		t.Fatalf("error = %v, want status %d", httpErr, status)
	}
}

// Luồng TOEIC_STRICT với đồng hồ giả. Layout từ seedTOEIC: trang 0 = Part 2 (gap 5s),
// trang 1 = group Part 3 (gap 8s), trang 2 = Part 5 (đọc). Grace mặc định 20s, audio 0ms.
func TestStrictAttemptPages(t *testing.T) {
	db := learningTestDB(t)
	ctx := context.Background()
	exam := seedTOEIC(t, db)
	svc := service.New(repository.NewRepository(db), &config.AppConfig{
		TOEICStrictFlowEnabled: true,
		TOEICListeningGapMs:    [4]int{5000, 5000, 8000, 8000},
	}, nil, testIdentityClient{}, nil, nil, nil, nil)

	created, httpErr := svc.CreateAttempt(ctx, "student-1", exam.ID, dto.CreateExamAttemptRequest{Mode: model.AttemptModeTest, ContextType: model.AttemptContextTypeStandalone})
	if httpErr != nil {
		t.Fatal(httpErr)
	}
	if created.Flow != string(model.AttemptFlowTOEICStrict) || created.Stage == nil || *created.Stage != "LISTENING" || created.Sections != nil {
		t.Fatalf("created = %#v", created)
	}

	var startedAt time.Time
	if err := db.Model(&model.Attempt{}).Select("listening_page_started_at").Where("id = ?", created.ID).Scan(&startedAt).Error; err != nil {
		t.Fatal(err)
	}
	now := startedAt
	service.SetClock(svc, func() time.Time { return now })

	_, httpErr = svc.GetAttemptQuestions(ctx, "student-1", created.ID)
	requireStatus(t, httpErr, http.StatusConflict)

	t.Run("navigator at first listening page", func(t *testing.T) {
		nav, httpErr := svc.GetAttemptNavigator(ctx, "student-1", created.ID)
		if httpErr != nil {
			t.Fatal(httpErr)
		}
		if len(nav.Pages) != 3 || nav.CurrentPage == nil || *nav.CurrentPage != 0 || nav.TotalQuestions != 4 {
			t.Fatalf("nav = %#v", nav)
		}
		if nav.Pages[0].Questions[0].Locked || !nav.Pages[1].Questions[0].Locked || !nav.Pages[2].Questions[0].Locked {
			t.Fatal("only page 0 should be unlocked")
		}
		if nav.Pages[0].Questions[0].Number != 1 || nav.Pages[1].Questions[1].Number != 3 || nav.Pages[2].Questions[0].Number != 4 {
			t.Fatal("questions without Number fall back to position in layout")
		}
	})

	t.Run("page access while listening", func(t *testing.T) {
		page, httpErr := svc.GetAttemptPage(ctx, "student-1", created.ID, 0)
		if httpErr != nil {
			t.Fatal(httpErr)
		}
		if page.Part == nil || *page.Part != "2" || page.Listening == nil || page.Listening.GapMs != 5000 {
			t.Fatalf("page = %#v", page)
		}
		if q := page.Questions[0]; q.Content != "" || q.Options[0].Content != nil || q.Options[0].Key != "A" {
			t.Fatalf("part 2 content must be hidden: %#v", q)
		}
		_, httpErr = svc.GetAttemptPage(ctx, "student-1", created.ID, 1)
		requireStatus(t, httpErr, http.StatusConflict)
		_, httpErr = svc.GetAttemptPage(ctx, "student-1", created.ID, 2)
		requireStatus(t, httpErr, http.StatusConflict)
		_, httpErr = svc.GetAttemptPage(ctx, "student-1", created.ID, 9)
		requireStatus(t, httpErr, http.StatusNotFound)
	})

	t.Run("lazy advance to the group page and resume after F5", func(t *testing.T) {
		pageOneStart := startedAt.Add(25 * time.Second) // 0 + 5s + 20s
		now = pageOneStart.Add(3 * time.Second)
		page, httpErr := svc.GetAttemptPage(ctx, "student-1", created.ID, 1)
		if httpErr != nil {
			t.Fatal(httpErr)
		}
		if len(page.Questions) != 2 || page.Group == nil || page.Listening.ElapsedMs != 3000 || page.Questions[0].Content == "" {
			t.Fatalf("page = %#v", page)
		}
		if page.Group.Title != "Part 3" {
			t.Fatalf("group = %#v", page.Group)
		}
		_, httpErr = svc.GetAttemptPage(ctx, "student-1", created.ID, 0)
		requireStatus(t, httpErr, http.StatusConflict)
	})

	t.Run("reading opens after the last listening page", func(t *testing.T) {
		now = startedAt.Add(25*time.Second + 28*time.Second + time.Millisecond)
		nav, httpErr := svc.GetAttemptNavigator(ctx, "student-1", created.ID)
		if httpErr != nil {
			t.Fatal(httpErr)
		}
		if nav.Stage == nil || *nav.Stage != "READING" || nav.CurrentPage != nil {
			t.Fatalf("nav = %#v", nav)
		}
		page, httpErr := svc.GetAttemptPage(ctx, "student-1", created.ID, 2)
		if httpErr != nil {
			t.Fatal(httpErr)
		}
		if page.Listening != nil || page.Questions[0].Content == "" {
			t.Fatalf("reading page = %#v", page)
		}
		_, httpErr = svc.GetAttemptPage(ctx, "student-1", created.ID, 1)
		requireStatus(t, httpErr, http.StatusConflict)
	})
}

// Flow FREE (cờ tắt): navigator không khóa trang nào và GET /questions vẫn chạy.
func TestFreeAttemptNavigator(t *testing.T) {
	db := learningTestDB(t)
	ctx := context.Background()
	exam := seedTOEIC(t, db)
	svc := newAttemptService(db)

	created, httpErr := svc.CreateAttempt(ctx, "student-1", exam.ID, dto.CreateExamAttemptRequest{Mode: model.AttemptModeTest, ContextType: model.AttemptContextTypeStandalone})
	if httpErr != nil {
		t.Fatal(httpErr)
	}
	if created.Flow != "FREE" || created.Sections == nil {
		t.Fatalf("created = %#v", created)
	}
	if _, httpErr := svc.GetAttemptQuestions(ctx, "student-1", created.ID); httpErr != nil {
		t.Fatal(httpErr)
	}
	nav, httpErr := svc.GetAttemptNavigator(ctx, "student-1", created.ID)
	if httpErr != nil {
		t.Fatal(httpErr)
	}
	for _, page := range nav.Pages {
		for _, question := range page.Questions {
			if question.Locked {
				t.Fatalf("free flow question %d locked", question.ID)
			}
		}
	}
	if _, httpErr := svc.GetAttemptPage(ctx, "student-1", created.ID, 2); httpErr != nil {
		t.Fatal(httpErr)
	}
}
