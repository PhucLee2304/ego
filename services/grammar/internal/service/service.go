package service

import (
	"context"
	"encoding/json"
	"errors"

	"ego/services/grammar/internal/dto"
	"ego/services/grammar/internal/model"
	"ego/services/grammar/internal/repository"
)

var ErrNotFound = errors.New("record not found")

type Service interface {
	// Curriculum
	GetCurriculum(ctx context.Context, userID string) (*dto.CurriculumResponse, error)

	// Lesson
	GetUnit(ctx context.Context, userID string, unitID uint) (*dto.UnitDetailResponse, error)

	// Progress
	SubmitProgress(ctx context.Context, userID string, unitID uint, completed bool) (*dto.ProgressResponse, error)
}

type service struct {
	repo *repository.Repository
}

func New(repo *repository.Repository) Service {
	return &service{repo: repo}
}

// ---- Curriculum ----

func (s *service) GetCurriculum(ctx context.Context, userID string) (*dto.CurriculumResponse, error) {
	units, err := s.repo.ListUnits(ctx)
	if err != nil {
		return nil, err
	}

	// Build progress map
	progressMap := make(map[uint]bool)
	if userID != "" {
		progList, err := s.repo.ListProgress(ctx, userID)
		if err != nil {
			return nil, err
		}
		for _, p := range progList {
			progressMap[p.UnitID] = p.Completed
		}
	}

	summaries := make([]dto.UnitSummaryResponse, 0, len(units))
	for _, u := range units {
		summaries = append(summaries, dto.UnitSummaryResponse{
			ID:        u.ID,
			Unit:      u.Unit,
			Title:     u.Title,
			Completed: progressMap[u.ID],
			Total:     1, // each unit has 1 practice set
		})
	}

	return &dto.CurriculumResponse{
		Data:  summaries,
		Total: len(summaries),
	}, nil
}

// ---- Lesson ----

func (s *service) GetUnit(ctx context.Context, userID string, unitID uint) (*dto.UnitDetailResponse, error) {
	unit, err := s.repo.GetUnitByID(ctx, unitID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	// Collect examples
	examples := make([]string, 0, len(unit.Examples))
	for _, e := range unit.Examples {
		examples = append(examples, e.Content)
	}

	// Collect practices
	practices := make([]dto.PracticeResponse, 0, len(unit.Practices))
	for _, p := range unit.Practices {
		var opts []string
		_ = json.Unmarshal([]byte(p.Options), &opts)
		if opts == nil {
			opts = []string{}
		}
		practices = append(practices, dto.PracticeResponse{
			ID:          p.ID,
			Question:    p.Question,
			Options:     opts,
			Correct:     p.Correct,
			Explanation: p.Explanation,
			SortOrder:   p.SortOrder,
		})
	}

	// Get progress
	completed := false
	if userID != "" {
		prog, err := s.repo.GetProgress(ctx, userID, unitID)
		if err != nil {
			return nil, err
		}
		if prog != nil {
			completed = prog.Completed
		}
	}

	return &dto.UnitDetailResponse{
		ID:        unit.ID,
		Unit:      unit.Unit,
		Title:     unit.Title,
		Theory:    unit.Theory,
		Examples:  examples,
		Practices: practices,
		Completed: completed,
	}, nil
}

// ---- Progress ----

func (s *service) SubmitProgress(ctx context.Context, userID string, unitID uint, completed bool) (*dto.ProgressResponse, error) {
	// Verify unit exists
	exists, err := s.repo.UnitExists(ctx, unitID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNotFound
	}

	prog, err := s.repo.UpsertProgress(ctx, userID, unitID, completed)
	if err != nil {
		return nil, err
	}

	return mapProgressToResponse(prog), nil
}

// ---- Mapper ----

func mapProgressToResponse(p *model.UserUnitProgress) *dto.ProgressResponse {
	return &dto.ProgressResponse{
		ID:        p.ID,
		UserID:    p.UserID,
		UnitID:    p.UnitID,
		Completed: p.Completed,
		UpdatedAt: p.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
