package service

import (
	"context"
	"errors"
	"net/http"

	"ego/platform/httpx"
	"ego/services/knowledge/internal/dto"
	"ego/services/knowledge/internal/model"

	"gorm.io/gorm"
)

func (s *service) GetUnits(ctx context.Context, userID string) (*dto.GetUnitsResponse, *httpx.HTTPError) {
	units, err := s.repo.GetUnits(ctx, userID)
	if err != nil {
		return nil, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to get units", err)
	}

	unitDTOs := make([]*dto.UnitSummary, len(units))
	for i, unit := range units {
		unitDTOs[i] = dto.ToUnitSummary(unit)
	}
	return &dto.GetUnitsResponse{Data: unitDTOs, Total: len(unitDTOs)}, nil
}

func (s *service) GetUnitByID(ctx context.Context, userID string, id uint) (*dto.Unit, *httpx.HTTPError) {
	unit, err := s.repo.GetUnitByID(ctx, userID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, httpx.NewError(http.StatusNotFound, "[NOT_FOUND] Unit not found", err)
		}
		return nil, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to get unit", err)
	}
	return dto.ToUnit(unit), nil
}

func (s *service) UpdateProgress(ctx context.Context, userID string, id uint, body dto.UpdateProgressBody) (*dto.Progress, *httpx.HTTPError) {
	progress := &model.UserUnitProgress{UserID: userID, UnitID: id, Completed: body.Completed}
	if err := s.repo.UpsertProgress(ctx, progress); err != nil {
		if errors.Is(err, gorm.ErrForeignKeyViolated) {
			return nil, httpx.NewError(http.StatusNotFound, "[NOT_FOUND] Unit not found", err)
		}
		return nil, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to update progress", err)
	}
	return dto.ToProgress(progress), nil
}
