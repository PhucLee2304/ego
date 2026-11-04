package repository

import (
	"context"

	"ego/services/learning/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ExamFilter lọc danh sách đề public; Search và Year để trống thì bỏ qua.
type ExamFilter struct {
	Type   model.ExamType
	Search string
	Year   *int
}

func (r *repository) GetListExam(ctx context.Context, filter ExamFilter, limit, offset int32) ([]*model.Exam, int64, error) {
	var exams []*model.Exam
	var total int64
	query := r.db.WithContext(ctx).Model(&model.Exam{}).Where("type = ? AND is_public = ?", filter.Type, true)
	if filter.Search != "" {
		query = query.Where("title ILIKE ?", "%"+filter.Search+"%")
	}
	if filter.Year != nil {
		query = query.Where("year = ?", *filter.Year)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.
		Order("id ASC").
		Limit(int(limit)).
		Offset(int(offset)).
		Find(&exams).Error; err != nil {
		return nil, 0, err
	}

	return exams, total, nil
}

// GetExamYears trả các năm (khác null) của đề public thuộc loại này, mới nhất trước.
func (r *repository) GetExamYears(ctx context.Context, examType model.ExamType) ([]int, error) {
	years := []int{}
	err := r.db.WithContext(ctx).Model(&model.Exam{}).
		Where("type = ? AND is_public = ? AND year IS NOT NULL", examType, true).
		Distinct("year").Order("year DESC").Pluck("year", &years).Error
	return years, err
}

func (r *repository) GetExamByID(ctx context.Context, id uint) (*model.Exam, error) {
	var exam model.Exam
	// Public exam routes intentionally exclude private classroom exams. Private
	// content is exposed only through an authorized classroom attempt.
	if err := r.db.WithContext(ctx).
		Preload("Sections", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		Preload("Sections.Groups", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		Preload("Sections.Groups.Questions", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		Preload("Sections.Questions", func(db *gorm.DB) *gorm.DB {
			return db.Where("group_id IS NULL").Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		Where("id = ? AND is_public = ?", id, true).
		First(&exam).Error; err != nil {
		return nil, err
	}

	return &exam, nil
}

func (r *repository) CreateExam(ctx context.Context, exam *model.Exam) error {
	return r.db.WithContext(ctx).Create(exam).Error
}

func (r *repository) GetQuestionsByExamID(ctx context.Context, id uint) (*model.Exam, error) {
	var exam model.Exam
	if err := r.db.WithContext(ctx).
		Preload("Sections", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		Preload("Sections.Groups", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		Preload("Sections.Groups.Questions", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		Preload("Sections.Groups.Questions.Options", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		Preload("Sections.Questions", func(db *gorm.DB) *gorm.DB {
			return db.Where("group_id IS NULL").Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		Preload("Sections.Questions.Options", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		First(&exam, id).Error; err != nil {
		return nil, err
	}

	return &exam, nil
}
