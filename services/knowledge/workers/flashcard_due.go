package workers

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	notifications "ego/api/gen/go/notifications"
	"ego/platform/logger"
	"ego/services/knowledge/internal/model"
)

type FlashcardReminderRepository interface {
	DueFlashcardsForNotification(ctx context.Context, now time.Time, limit int) ([]*model.Flashcard, error)
	MarkFlashcardNotified(ctx context.Context, id uint, dueAt time.Time) error
}

func StartFlashcardDue(ctx context.Context, repo FlashcardReminderRepository, client notifications.NotificationServiceClient, interval time.Duration, batchSize int) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			if _, err := ProcessFlashcardDue(ctx, repo, client, time.Now().UTC(), batchSize); err != nil {
				logger.Log.Error().Err(err).Msg("[FLASHCARDS] Failed to dispatch due reminders")
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func ProcessFlashcardDue(ctx context.Context, repo FlashcardReminderRepository, client notifications.NotificationServiceClient, now time.Time, batchSize int) (int, error) {
	cards, err := repo.DueFlashcardsForNotification(ctx, now, batchSize)
	if err != nil {
		return 0, err
	}
	sent := 0
	var failures []error
	for _, card := range cards {
		if err := ctx.Err(); err != nil {
			return sent, err
		}
		word := card.Word.Word
		if word == "" {
			word = "flashcard"
		}
		request := &notifications.CreateNotificationsRequest{
			Recipients:     []string{card.UserID},
			Title:          "Đến hạn ôn flashcard",
			Body:           fmt.Sprintf("Thẻ %q đã đến hạn ôn tập.", word),
			SourceService:  "knowledge",
			IdempotencyKey: fmt.Sprintf("flashcard-due:%d:%d", card.ID, card.NextReviewAt.UTC().UnixNano()),
			Data: map[string]string{
				"type":   "FLASHCARD_DUE",
				"cardId": strconv.FormatUint(uint64(card.ID), 10),
				"deckId": strconv.FormatUint(uint64(card.DeckID), 10),
				"wordId": strconv.FormatUint(uint64(card.WordID), 10),
			},
		}
		callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		_, err := client.CreateNotifications(callCtx, request)
		cancel()
		if err != nil {
			failures = append(failures, fmt.Errorf("card %d: %w", card.ID, err))
			continue
		}
		if err := repo.MarkFlashcardNotified(ctx, card.ID, card.NextReviewAt); err != nil {
			failures = append(failures, fmt.Errorf("mark card %d: %w", card.ID, err))
			continue
		}
		sent++
	}
	return sent, errors.Join(failures...)
}
