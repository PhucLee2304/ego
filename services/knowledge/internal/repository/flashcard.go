package repository

import (
	"context"
	"time"

	platformdb "ego/platform/db"
	"ego/platform/httpx"
	"ego/services/knowledge/internal/dto"
	"ego/services/knowledge/internal/model"

	"gorm.io/gorm"
)

// ─── Deck ─────────────────────────────────────────────────────────────────────

// CreateDeck tạo mới một FlashcardDeck trong DB.
func (r *repository) CreateDeck(ctx context.Context, deck *model.FlashcardDeck) error {
	return r.db.WithContext(ctx).Create(deck).Error
}

// GetDecksByUser trả danh sách tất cả deck của một người dùng.
func (r *repository) GetDecksByUser(ctx context.Context, userID string) ([]*model.FlashcardDeck, error) {
	var decks []*model.FlashcardDeck
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at ASC").
		Find(&decks).Error; err != nil {
		return nil, err
	}
	return decks, nil
}

// GetDeckByID lấy một deck theo ID, kiểm tra ownership.
func (r *repository) GetDeckByID(ctx context.Context, userID string, id uint) (*model.FlashcardDeck, error) {
	var deck model.FlashcardDeck
	if err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		First(&deck).Error; err != nil {
		return nil, err
	}
	return &deck, nil
}

// UpdateDeck cập nhật tên/mô tả/màu của deck, kiểm tra ownership.
// Chỉ cập nhật field không nil. Nếu không tìm thấy trả gorm.ErrRecordNotFound.
func (r *repository) UpdateDeck(ctx context.Context, userID string, id uint, name string, description, color *string) error {
	updates := map[string]any{"name": name}
	if description != nil {
		updates["description"] = description
	}
	if color != nil {
		updates["color"] = color
	}

	result := r.db.WithContext(ctx).
		Model(&model.FlashcardDeck{}).
		Where("id = ? AND user_id = ?", id, userID).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// DeleteDeck xóa mềm deck và toàn bộ thẻ bên trong theo transaction.
// Kiểm tra ownership trước khi xóa.
func (r *repository) DeleteDeck(ctx context.Context, userID string, id uint) error {
	return platformdb.WithTx(ctx, r.db, func(tx *gorm.DB) error {
		// Kiểm tra ownership
		var count int64
		if err := tx.Model(&model.FlashcardDeck{}).
			Where("id = ? AND user_id = ?", id, userID).
			Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return gorm.ErrRecordNotFound
		}
		// Xóa thẻ bên trong (cascade sẽ xử lý ở DB level, nhưng soft-delete cần làm thủ công)
		if err := tx.Where("deck_id = ? AND user_id = ?", id, userID).
			Delete(&model.Flashcard{}).Error; err != nil {
			return err
		}
		// Xóa deck
		result := tx.Where("id = ? AND user_id = ?", id, userID).Delete(&model.FlashcardDeck{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

// CountDueCardsByDecks trả số thẻ đến hạn (NextReviewAt <= now) theo từng deck.
// Kết quả là map[deckID]count; deck không có thẻ đến hạn không xuất hiện trong map.
func (r *repository) CountDueCardsByDecks(ctx context.Context, userID string, deckIDs []uint, now time.Time) (map[uint]int, error) {
	if len(deckIDs) == 0 {
		return map[uint]int{}, nil
	}

	type row struct {
		DeckID uint
		Count  int
	}
	var rows []row
	if err := r.db.WithContext(ctx).
		Model(&model.Flashcard{}).
		Select("deck_id, COUNT(*) AS count").
		Where("user_id = ? AND deck_id IN ? AND next_review_at <= ?", userID, deckIDs, now).
		Group("deck_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	result := make(map[uint]int, len(rows))
	for _, row := range rows {
		result[row.DeckID] = row.Count
	}
	return result, nil
}

// preloadFlashcardWord nạp đầy đủ thông tin Word bao gồm Phonetics, Meanings và Definitions cho thẻ.
func preloadFlashcardWord(db *gorm.DB) *gorm.DB {
	return db.
		Preload("Word").
		Preload("Word.Phonetics", func(tx *gorm.DB) *gorm.DB {
			return tx.Order("id")
		}).
		Preload("Word.Meanings", func(tx *gorm.DB) *gorm.DB {
			return tx.Order("id")
		}).
		Preload("Word.Meanings.Definitions", func(tx *gorm.DB) *gorm.DB {
			return tx.Order("id")
		})
}

// ─── Flashcard ────────────────────────────────────────────────────────────────

// GetCardsByDeck trả danh sách thẻ trong một deck, có phân trang, kèm Preload Word đầy đủ.
func (r *repository) GetCardsByDeck(ctx context.Context, userID string, deckID uint, limit, offset int32) ([]*model.Flashcard, int64, error) {
	var cards []*model.Flashcard
	var total int64

	db := r.db.WithContext(ctx).Model(&model.Flashcard{}).
		Where("user_id = ? AND deck_id = ?", userID, deckID)

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := preloadFlashcardWord(db).
		Order("created_at DESC").
		Limit(int(limit)).
		Offset(int(offset)).
		Find(&cards).Error; err != nil {
		return nil, 0, err
	}
	return cards, total, nil
}

// CreateFlashcard tạo thẻ mới và preload Word sau khi tạo.
func (r *repository) CreateFlashcard(ctx context.Context, fc *model.Flashcard) error {
	if err := r.db.WithContext(ctx).Create(fc).Error; err != nil {
		return err
	}
	return preloadFlashcardWord(r.db.WithContext(ctx)).First(fc, fc.ID).Error
}

// GetFlashcardByID lấy thẻ theo ID, kiểm tra ownership, preload Word.
func (r *repository) GetFlashcardByID(ctx context.Context, userID string, id uint) (*model.Flashcard, error) {
	var fc model.Flashcard
	if err := preloadFlashcardWord(r.db.WithContext(ctx)).
		Where("id = ? AND user_id = ?", id, userID).
		First(&fc).Error; err != nil {
		return nil, err
	}
	return &fc, nil
}

// GetWordSaveStatus trả danh sách DeckID chứa từ wordID của người dùng.
func (r *repository) GetWordSaveStatus(ctx context.Context, userID string, wordID uint) ([]uint, error) {
	var deckIDs []uint
	if err := r.db.WithContext(ctx).
		Model(&model.Flashcard{}).
		Select("deck_id").
		Where("user_id = ? AND word_id = ?", userID, wordID).
		Pluck("deck_id", &deckIDs).Error; err != nil {
		return nil, err
	}
	return deckIDs, nil
}

// GetStudyCards lấy tối đa limit thẻ đến hạn trong một deck, Preload Word đầy đủ.
func (r *repository) GetStudyCards(ctx context.Context, userID string, deckID uint, limit int, now time.Time) ([]*model.Flashcard, error) {
	var cards []*model.Flashcard
	if err := preloadFlashcardWord(r.db.WithContext(ctx)).
		Where("user_id = ? AND deck_id = ? AND next_review_at <= ?", userID, deckID, now).
		Order("next_review_at ASC").
		Limit(limit).
		Find(&cards).Error; err != nil {
		return nil, err
	}
	return cards, nil
}

// GetAllDueCards lấy tối đa limit thẻ đến hạn trên toàn bộ deck của người dùng.
func (r *repository) GetAllDueCards(ctx context.Context, userID string, limit int, now time.Time) ([]*model.Flashcard, error) {
	var cards []*model.Flashcard
	if err := preloadFlashcardWord(r.db.WithContext(ctx)).
		Where("user_id = ? AND next_review_at <= ?", userID, now).
		Order("next_review_at ASC").
		Limit(limit).
		Find(&cards).Error; err != nil {
		return nil, err
	}
	return cards, nil
}

// DueFlashcardsForNotification returns reviewed cards whose next due time has
// not yet produced a reminder. New cards are excluded to avoid save-time alerts.
func (r *repository) DueFlashcardsForNotification(ctx context.Context, now time.Time, limit int) ([]*model.Flashcard, error) {
	var cards []*model.Flashcard
	err := r.db.WithContext(ctx).Model(&model.Flashcard{}).
		Preload("Word", func(db *gorm.DB) *gorm.DB { return db.Select("id", "word") }).
		Where("last_reviewed_at IS NOT NULL AND next_review_at <= ? AND (last_notified_review_at IS NULL OR last_notified_review_at < next_review_at)", now).
		Order("next_review_at ASC").Limit(limit).Find(&cards).Error
	return cards, err
}

// MarkFlashcardNotified updates only the review cycle that was sent. A review
// racing with delivery must leave the new due time eligible for a later alert.
func (r *repository) MarkFlashcardNotified(ctx context.Context, id uint, dueAt time.Time) error {
	return r.db.WithContext(ctx).Model(&model.Flashcard{}).
		Where("id = ? AND next_review_at = ? AND (last_notified_review_at IS NULL OR last_notified_review_at < next_review_at)", id, dueAt).
		UpdateColumn("last_notified_review_at", dueAt).Error
}

// UpdateFlashcardNote ghi ghi chú của thẻ (kiểm tra ownership) và trả thẻ sau khi cập nhật.
func (r *repository) UpdateFlashcardNote(ctx context.Context, userID string, id uint, note *string) (*model.Flashcard, error) {
	result := r.db.WithContext(ctx).
		Model(&model.Flashcard{}).
		Where("id = ? AND user_id = ?", id, userID).
		Update("note", note)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return r.GetFlashcardByID(ctx, userID, id)
}

// SaveWordToDecks đặt các deck chứa từ của người dùng đúng bằng deckIDs (cộng deck mới
// newDeckName nếu khác rỗng) trong một transaction: thêm thẻ vào deck còn thiếu, gỡ thẻ khỏi
// deck không còn được chọn. deckIDs rỗng là bỏ lưu từ. Deck không thuộc người dùng trả
// gorm.ErrRecordNotFound.
func (r *repository) SaveWordToDecks(ctx context.Context, userID string, wordID uint, deckIDs []uint, newDeckName string, note *string) error {
	return platformdb.WithTx(ctx, r.db.WithContext(ctx), func(tx *gorm.DB) error {
		if err := tx.Select("id").First(&model.Word{}, wordID).Error; err != nil {
			return err
		}

		targets := make([]uint, 0, len(deckIDs)+1)
		seen := make(map[uint]bool, len(deckIDs))
		for _, deckID := range deckIDs {
			if seen[deckID] {
				continue
			}
			seen[deckID] = true
			if err := tx.Select("id").Where("id = ? AND user_id = ?", deckID, userID).
				First(&model.FlashcardDeck{}).Error; err != nil {
				return err
			}
			targets = append(targets, deckID)
		}
		if newDeckName != "" {
			deck := &model.FlashcardDeck{UserID: userID, Name: newDeckName}
			if err := tx.Create(deck).Error; err != nil {
				return err
			}
			targets = append(targets, deck.ID)
		}

		var existing []model.Flashcard
		if err := tx.Select("id", "deck_id").
			Where("user_id = ? AND word_id = ?", userID, wordID).
			Find(&existing).Error; err != nil {
			return err
		}
		wanted := make(map[uint]bool, len(targets))
		for _, deckID := range targets {
			wanted[deckID] = true
		}
		saved := make(map[uint]bool, len(existing))
		for _, card := range existing {
			if wanted[card.DeckID] {
				saved[card.DeckID] = true
				continue
			}
			if err := tx.Delete(&model.Flashcard{}, card.ID).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.FlashcardDeck{}).Where("id = ?", card.DeckID).
				UpdateColumn("card_count", gorm.Expr("GREATEST(card_count - 1, 0)")).Error; err != nil {
				return err
			}
		}

		now := time.Now()
		for _, deckID := range targets {
			if saved[deckID] {
				continue
			}
			card := &model.Flashcard{
				UserID:       userID,
				DeckID:       deckID,
				WordID:       wordID,
				Note:         note,
				Status:       model.FlashcardStatusNew,
				EaseFactor:   2.5,
				NextReviewAt: now,
			}
			if err := tx.Create(card).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.FlashcardDeck{}).Where("id = ?", deckID).
				UpdateColumn("card_count", gorm.Expr("card_count + 1")).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ReviewFlashcard ghi trạng thái SM-2 mới của thẻ và log ôn tập trong một transaction.
func (r *repository) ReviewFlashcard(ctx context.Context, fc *model.Flashcard, log *model.FlashcardReviewLog) error {
	return platformdb.WithTx(ctx, r.db.WithContext(ctx), func(tx *gorm.DB) error {
		if err := tx.Model(fc).
			Select("status", "interval_days", "ease_factor", "repetitions", "next_review_at", "last_reviewed_at").
			Updates(fc).Error; err != nil {
			return err
		}
		return tx.Create(log).Error
	})
}

// DeleteFlashcard xóa mềm một thẻ, kiểm tra ownership.
func (r *repository) DeleteFlashcard(ctx context.Context, userID string, id uint) error {
	return platformdb.WithTx(ctx, r.db.WithContext(ctx), func(tx *gorm.DB) error {
		var fc model.Flashcard
		if err := tx.Where("id = ? AND user_id = ?", id, userID).First(&fc).Error; err != nil {
			return err
		}
		if err := tx.Delete(&fc).Error; err != nil {
			return err
		}
		// Giữ card_count của deck khớp với số thẻ còn lại.
		return tx.Model(&model.FlashcardDeck{}).
			Where("id = ?", fc.DeckID).
			UpdateColumn("card_count", gorm.Expr("GREATEST(card_count - 1, 0)")).Error
	})
}

// ─── Stats ────────────────────────────────────────────────────────────────────

// GetFlashcardStats trả thống kê tổng quan thẻ của người dùng.
// Streak được tính là số ngày liên tiếp (kết thúc từ hôm nay hoặc hôm qua) có ít nhất 1 review.
func (r *repository) GetFlashcardStats(ctx context.Context, userID string) (*dto.FlashcardUserStats, error) {
	type cardCounts struct {
		Total    int
		Mastered int
	}
	var counts cardCounts
	if err := r.db.WithContext(ctx).
		Model(&model.Flashcard{}).
		Select("COUNT(*) AS total, COUNT(CASE WHEN status = 'mastered' THEN 1 END) AS mastered").
		Where("user_id = ?", userID).
		Scan(&counts).Error; err != nil {
		return nil, err
	}

	// Cắt ngày theo giờ Việt Nam như các API thống kê khác.
	now := time.Now().In(httpx.StatsLocation)
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, httpx.StatsLocation)
	endOfToday := startOfToday.AddDate(0, 0, 1)

	var dueToday int64
	if err := r.db.WithContext(ctx).
		Model(&model.Flashcard{}).
		Where("user_id = ? AND next_review_at < ?", userID, endOfToday).
		Count(&dueToday).Error; err != nil {
		return nil, err
	}

	streak, reviewedToday, err := r.calcStreak(ctx, userID, now)
	if err != nil {
		return nil, err
	}

	return &dto.FlashcardUserStats{
		TotalCards:    counts.Total,
		MasteredCards: counts.Mastered,
		DueToday:      int(dueToday),
		Streak:        streak,
		ReviewedToday: reviewedToday,
	}, nil
}

// calcStreak đếm số ngày liên tiếp có ít nhất 1 review, tính ngược từ hôm nay (hoặc hôm qua)
// theo ngày địa phương Việt Nam.
func (r *repository) calcStreak(ctx context.Context, userID string, now time.Time) (int, bool, error) {
	var days []string
	if err := r.db.WithContext(ctx).
		Model(&model.FlashcardReviewLog{}).
		Select("to_char(created_at AT TIME ZONE ?, 'YYYY-MM-DD') AS d", httpx.StatsTimeZone).
		Where("user_id = ?", userID).
		Group("d").
		Order("d DESC").
		Pluck("d", &days).Error; err != nil {
		return 0, false, err
	}
	dates := make([]time.Time, 0, len(days))
	for _, day := range days {
		date, err := time.ParseInLocation(time.DateOnly, day, httpx.StatsLocation)
		if err != nil {
			return 0, false, err
		}
		dates = append(dates, date)
	}
	streak, reviewedToday := streakFromReviewDays(dates, now)
	return streak, reviewedToday, nil
}

// dates already contain local calendar dates from PostgreSQL. Use their date
// components directly so the database driver's time zone cannot shift a day.
func streakFromReviewDays(dates []time.Time, now time.Time) (int, bool) {
	if len(dates) == 0 {
		return 0, false
	}
	localNow := now.In(httpx.StatsLocation)
	today := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, httpx.StatsLocation)
	day := func(value time.Time) time.Time {
		return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, httpx.StatsLocation)
	}
	first := day(dates[0])
	reviewedToday := first.Equal(today)
	if !reviewedToday && !first.Equal(today.AddDate(0, 0, -1)) {
		return 0, false
	}
	streak := 1
	previous := first
	for _, value := range dates[1:] {
		current := day(value)
		if current.Equal(previous) {
			continue
		}
		if !current.Equal(previous.AddDate(0, 0, -1)) {
			break
		}
		streak++
		previous = current
	}
	return streak, reviewedToday
}

// ─── Legacy ───────────────────────────────────────────────────────────────────
// GetFlashcards giữ lại để không break interface cũ cho đến khi service layer được cập nhật.

func (r *repository) GetFlashcards(ctx context.Context, userID string, limit, offset int32) ([]*model.Flashcard, int64, error) {
	var cards []*model.Flashcard
	var total int64
	db := r.db.WithContext(ctx).Model(&model.Flashcard{}).Where("user_id = ?", userID)
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := preloadFlashcardWord(db).Order("created_at DESC").Limit(int(limit)).Offset(int(offset)).Find(&cards).Error; err != nil {
		return nil, 0, err
	}
	return cards, total, nil
}
