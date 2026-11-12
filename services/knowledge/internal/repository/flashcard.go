package repository

import (
	"context"
	"time"

	platformdb "ego/platform/db"
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

// IsWordInDeck kiểm tra từ đã có trong deck chưa.
func (r *repository) IsWordInDeck(ctx context.Context, userID string, deckID, wordID uint) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.Flashcard{}).
		Where("user_id = ? AND deck_id = ? AND word_id = ?", userID, deckID, wordID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
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

// UpdateFlashcardSRS cập nhật các field SRS của thẻ (SM-2).
func (r *repository) UpdateFlashcardSRS(ctx context.Context, fc *model.Flashcard) error {
	return r.db.WithContext(ctx).
		Model(fc).
		Select("status", "interval_days", "ease_factor", "repetitions", "next_review_at", "last_reviewed_at").
		Updates(fc).Error
}

// DeleteFlashcard xóa mềm một thẻ, kiểm tra ownership.
func (r *repository) DeleteFlashcard(ctx context.Context, userID string, id uint) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		Delete(&model.Flashcard{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// UpdateDeckCardCount cộng/trừ card_count của deck bằng raw UPDATE để tránh race condition.
func (r *repository) UpdateDeckCardCount(ctx context.Context, deckID uint, delta int) error {
	return r.db.WithContext(ctx).
		Model(&model.FlashcardDeck{}).
		Where("id = ?", deckID).
		UpdateColumn("card_count", gorm.Expr("card_count + ?", delta)).Error
}

// ─── Review Log ───────────────────────────────────────────────────────────────

// CreateReviewLog ghi lại một lần ôn tập.
func (r *repository) CreateReviewLog(ctx context.Context, log *model.FlashcardReviewLog) error {
	return r.db.WithContext(ctx).Create(log).Error
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

	now := time.Now()
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endOfToday := startOfToday.Add(24 * time.Hour)

	var dueToday int64
	if err := r.db.WithContext(ctx).
		Model(&model.Flashcard{}).
		Where("user_id = ? AND next_review_at < ?", userID, endOfToday).
		Count(&dueToday).Error; err != nil {
		return nil, err
	}

	streak, err := r.calcStreak(ctx, userID, now)
	if err != nil {
		return nil, err
	}

	return &dto.FlashcardUserStats{
		TotalCards:    counts.Total,
		MasteredCards: counts.Mastered,
		DueToday:      int(dueToday),
		Streak:        streak,
	}, nil
}

// calcStreak đếm số ngày liên tiếp có ít nhất 1 review, tính ngược từ hôm nay (hoặc hôm qua).
// Dùng SQL để lấy danh sách ngày distinct từ review_logs, sau đó duyệt Go.
func (r *repository) calcStreak(ctx context.Context, userID string, now time.Time) (int, error) {
	// Lấy các ngày (DATE) distinct từ review_logs, sắp xếp giảm dần.
	var dates []time.Time
	if err := r.db.WithContext(ctx).
		Model(&model.FlashcardReviewLog{}).
		Select("DATE(created_at) AS d").
		Where("user_id = ?", userID).
		Group("DATE(created_at)").
		Order("d DESC").
		Pluck("d", &dates).Error; err != nil {
		return 0, err
	}
	if len(dates) == 0 {
		return 0, nil
	}

	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	yesterday := today.AddDate(0, 0, -1)

	// Streak phải bắt đầu từ hôm nay hoặc hôm qua.
	first := time.Date(dates[0].Year(), dates[0].Month(), dates[0].Day(), 0, 0, 0, 0, now.Location())
	if !first.Equal(today) && !first.Equal(yesterday) {
		return 0, nil
	}

	streak := 1
	prev := first
	for _, d := range dates[1:] {
		cur := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, now.Location())
		if prev.AddDate(0, 0, -1).Equal(cur) {
			streak++
			prev = cur
		} else {
			break
		}
	}
	return streak, nil
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
