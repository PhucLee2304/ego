package repository

import (
	"context"

	"ego/services/learning/internal/model"

	"gorm.io/gorm"
)

func (r *repository) GetPrivateAssignmentExam(ctx context.Context, id uint) (*model.Exam, error) {
	exam, err := r.GetQuestionsByExamID(ctx, id)
	if err != nil {
		return nil, err
	}
	if exam.IsPublic || exam.Type != model.ExamTypeAssignment {
		return nil, gorm.ErrRecordNotFound
	}
	return exam, nil
}

func (r *repository) ReplacePrivateExamQuestions(ctx context.Context, exam *model.Exam, questions []model.Question) error {
	if len(exam.Sections) != 1 {
		return gorm.ErrRecordNotFound
	}
	section := &exam.Sections[0]
	questionIDs := r.db.WithContext(ctx).Model(&model.Question{}).Select("id").Where("section_id = ?", section.ID)
	if err := r.db.WithContext(ctx).Where("question_id IN (?)", questionIDs).Delete(&model.Option{}).Error; err != nil {
		return err
	}
	if err := r.db.WithContext(ctx).Where("section_id = ?", section.ID).Delete(&model.Question{}).Error; err != nil {
		return err
	}
	for i := range questions {
		questions[i].SectionID = section.ID
	}
	if err := r.db.WithContext(ctx).Create(&questions).Error; err != nil {
		return err
	}
	return r.db.WithContext(ctx).Model(exam).Updates(map[string]any{
		"title": exam.Title, "description": exam.Description,
	}).Error
}

func (r *repository) CreateExamAudit(ctx context.Context, audit *model.ExamAudit) error {
	return r.db.WithContext(ctx).Create(audit).Error
}
