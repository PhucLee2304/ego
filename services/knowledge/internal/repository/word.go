package repository

import (
	"context"
	"strings"

	"ego/services/knowledge/internal/model"

	"gorm.io/gorm"
)

// likeEscaper thoát ký tự đặc biệt của LIKE để chỉ khớp theo tiền tố thật.
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// GetWords tìm từ theo tiền tố (cột word lưu chữ thường), sắp theo chữ cái.
func (r *repository) GetWords(ctx context.Context, query string, limit, offset int32) ([]*model.Word, int64, error) {
	var words []*model.Word
	var total int64

	db := r.db.WithContext(ctx).Model(&model.Word{})
	if query != "" {
		db = db.Where("word LIKE ?", likeEscaper.Replace(strings.ToLower(query))+"%")
	}
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := db.Order("word ASC").Limit(int(limit)).Offset(int(offset)).Find(&words).Error; err != nil {
		return nil, 0, err
	}
	return words, total, nil
}

// GetWordByID lấy từ kèm phát âm, nghĩa và định nghĩa, theo thứ tự seed (id).
func (r *repository) GetWordByID(ctx context.Context, id uint) (*model.Word, error) {
	var word model.Word
	err := r.db.WithContext(ctx).
		Preload("Phonetics", func(db *gorm.DB) *gorm.DB {
			return db.Order("id")
		}).
		Preload("Meanings", func(db *gorm.DB) *gorm.DB {
			return db.Order("id")
		}).
		Preload("Meanings.Definitions", func(db *gorm.DB) *gorm.DB {
			return db.Order("id")
		}).
		First(&word, id).Error
	if err != nil {
		return nil, err
	}
	return &word, nil
}
