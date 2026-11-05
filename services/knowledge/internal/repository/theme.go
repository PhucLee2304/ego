package repository

import (
	"context"

	"ego/services/knowledge/internal/model"
)

// GetThemes lấy mọi theme theo thứ tự hiển thị.
func (r *repository) GetThemes(ctx context.Context) ([]*model.Theme, error) {
	var themes []*model.Theme
	if err := r.db.WithContext(ctx).Order("order_index ASC").Order("id ASC").Find(&themes).Error; err != nil {
		return nil, err
	}
	return themes, nil
}

// GetThemeByID lấy theme kèm một trang từ của nó theo order_num. Từ được khớp
// sang bảng words theo chữ thường; từ không có trong từ điển trả về với ID = 0
// và phonetic NULL.
func (r *repository) GetThemeByID(ctx context.Context, id uint, limit, offset int32) (*model.Theme, []*model.Word, int64, error) {
	var theme model.Theme
	var words []*model.Word
	var total int64

	if err := r.db.WithContext(ctx).First(&theme, id).Error; err != nil {
		return nil, nil, 0, err
	}

	db := r.db.WithContext(ctx).Model(&model.ThemeWord{}).Where("theme_words.theme_id = ?", id)
	if err := db.Count(&total).Error; err != nil {
		return nil, nil, 0, err
	}
	if err := db.Select("COALESCE(words.id, 0) AS id, theme_words.word, words.phonetic").
		Joins("LEFT JOIN words ON words.word = LOWER(theme_words.word) AND words.deleted_at IS NULL").
		Order("theme_words.order_num ASC").
		Limit(int(limit)).
		Offset(int(offset)).
		Scan(&words).Error; err != nil {
		return nil, nil, 0, err
	}
	return &theme, words, total, nil
}
