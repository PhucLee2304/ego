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

func (r *repository) UpdateQuestionScoring(ctx context.Context, questionID uint, excluded bool, correctOptionID uint) error {
	if err := r.db.WithContext(ctx).Model(&model.Question{}).Where("id = ?", questionID).Update("excluded_from_score", excluded).Error; err != nil {
		return err
	}
	if err := r.db.WithContext(ctx).Model(&model.Option{}).Where("question_id = ? AND id <> ?", questionID, correctOptionID).Update("is_correct", false).Error; err != nil {
		return err
	}
	return r.db.WithContext(ctx).Model(&model.Option{}).Where("question_id = ? AND id = ?", questionID, correctOptionID).Update("is_correct", true).Error
}

func (r *repository) GetSubmittedAttemptsByExamID(ctx context.Context, examID uint) ([]*model.Attempt, error) {
	var attempts []*model.Attempt
	if err := r.db.WithContext(ctx).
		Preload("Answers").Preload("History").
		Preload("Exam.Sections").
		Preload("Exam.Sections.Questions", "group_id IS NULL").
		Preload("Exam.Sections.Questions.Options").
		Where("exam_id = ? AND status = ?", examID, model.AttemptStatusSubmitted).
		Find(&attempts).Error; err != nil {
		return nil, err
	}
	return attempts, nil
}

func (r *repository) GetExamAudits(ctx context.Context, examID uint) ([]*model.ExamAudit, error) {
	var audits []*model.ExamAudit
	if err := r.db.WithContext(ctx).Where("exam_id = ?", examID).Order("created_at DESC, id DESC").Find(&audits).Error; err != nil {
		return nil, err
	}
	return audits, nil
}
