package rpc

import (
	"context"
	examsClient "ego/api/gen/go/exams"
	"ego/services/exams/internal/dto"
	"ego/services/exams/internal/model"
	"ego/services/exams/internal/service"
	"time"
)

type server struct {
	examsClient.UnimplementedExamServiceServer
	service service.Service
}

func New(service service.Service) examsClient.ExamServiceServer {
	return &server{service: service}
}

func (s *server) CreatePrivateExam(ctx context.Context, req *examsClient.CreatePrivateExamRequest) (*examsClient.CreatePrivateExamResponse, error) {
	var description *string
	if req.Description != nil {
		value := req.GetDescription()
		description = &value
	}

	questions := make([]dto.CreatePrivateExamQuestionRequest, len(req.Questions))
	for i, question := range req.Questions {
		var explanation *string
		if question.Explanation != nil {
			value := question.GetExplanation()
			explanation = &value
		}
		questions[i] = dto.CreatePrivateExamQuestionRequest{
			Content:            question.Content,
			Explanation:        explanation,
			Options:            question.Options,
			CorrectOptionIndex: int(question.CorrectOptionIndex),
		}
	}

	exam, err := s.service.CreatePrivateExam(ctx, dto.CreatePrivateExamRequest{
		Title:       req.Title,
		Description: description,
		Questions:   questions,
	})
	if err != nil {
		return nil, err
	}

	resp := &examsClient.CreatePrivateExamResponse{
		Id:       uint64(exam.ID),
		Title:    exam.Title,
		Type:     string(exam.Type),
		IsPublic: exam.IsPublic,
	}
	if exam.Description != nil {
		resp.Description = exam.Description
	}
	return resp, nil
}

func (s *server) DeletePrivateExamRollback(ctx context.Context, req *examsClient.DeletePrivateExamRollbackRequest) (*examsClient.DeletePrivateExamRollbackResponse, error) {
	if err := s.service.DeletePrivateExamRollback(ctx, uint(req.Id)); err != nil {
		return nil, err
	}
	return &examsClient.DeletePrivateExamRollbackResponse{Deleted: true}, nil
}

func (s *server) SubmitAssignmentAttempts(ctx context.Context, req *examsClient.SubmitAssignmentAttemptsRequest) (*examsClient.SubmitAssignmentAttemptsResponse, error) {
	count, err := s.service.SubmitAssignmentAttempts(ctx, uint(req.AssignmentId))
	if err != nil {
		return nil, err
	}
	return &examsClient.SubmitAssignmentAttemptsResponse{SubmittedCount: int32(count)}, nil
}

func (s *server) GetAssignmentAttemptState(ctx context.Context, req *examsClient.GetAssignmentAttemptStateRequest) (*examsClient.GetAssignmentAttemptStateResponse, error) {
	active, total, err := s.service.GetAssignmentAttemptState(ctx, uint(req.AssignmentId))
	if err != nil {
		return nil, err
	}
	return &examsClient.GetAssignmentAttemptStateResponse{ActiveCount: active, TotalCount: total}, nil
}

func (s *server) UpdateAssignmentAttemptSchedule(ctx context.Context, req *examsClient.UpdateAssignmentAttemptScheduleRequest) (*examsClient.UpdateAssignmentAttemptScheduleResponse, error) {
	count, err := s.service.UpdateAssignmentAttemptSchedule(ctx, uint(req.AssignmentId), int(req.DurationMinutes), time.Unix(req.DueAtUnix, 0))
	if err != nil {
		return nil, err
	}
	return &examsClient.UpdateAssignmentAttemptScheduleResponse{UpdatedCount: count}, nil
}

func privateExamResponse(exam *model.Exam) *examsClient.PrivateExamResponse {
	response := &examsClient.PrivateExamResponse{Id: uint64(exam.ID), Title: exam.Title, Description: exam.Description}
	for _, section := range exam.Sections {
		for _, question := range section.Questions {
			item := &examsClient.PrivateExamQuestion{Id: uint64(question.ID), Content: question.Content, ExcludedFromScore: question.ExcludedFromScore}
			if question.Explanation != "" {
				item.Explanation = &question.Explanation
			}
			for index, option := range question.Options {
				item.OptionIds = append(item.OptionIds, uint64(option.ID))
				if option.Content != nil {
					item.Options = append(item.Options, *option.Content)
				} else {
					item.Options = append(item.Options, "")
				}
				if option.IsCorrect {
					item.CorrectOptionIndex = int32(index)
				}
			}
			response.Questions = append(response.Questions, item)
		}
	}
	return response
}

func (s *server) GetPrivateExam(ctx context.Context, req *examsClient.GetPrivateExamRequest) (*examsClient.PrivateExamResponse, error) {
	exam, err := s.service.GetPrivateExam(ctx, uint(req.Id))
	if err != nil {
		return nil, err
	}
	return privateExamResponse(exam), nil
}

func (s *server) UpdatePrivateExam(ctx context.Context, req *examsClient.UpdatePrivateExamRequest) (*examsClient.PrivateExamResponse, error) {
	questions := make([]dto.UpdatePrivateExamQuestionRequest, len(req.Questions))
	for i, question := range req.Questions {
		optionIDs := make([]uint, len(question.OptionIds))
		for index, optionID := range question.OptionIds {
			optionIDs[index] = uint(optionID)
		}
		questions[i] = dto.UpdatePrivateExamQuestionRequest{ID: uint(question.Id), Content: question.Content, Explanation: question.Explanation, Options: question.Options, CorrectOptionIndex: int(question.CorrectOptionIndex), ExcludedFromScore: question.ExcludedFromScore, OptionIDs: optionIDs}
	}
	exam, err := s.service.UpdatePrivateExam(ctx, uint(req.Id), dto.UpdatePrivateExamRequest{Title: req.Title, Description: req.Description, Questions: questions, CorrectionOnly: req.CorrectionOnly, ActorID: req.ActorId, Reason: req.Reason})
	if err != nil {
		return nil, err
	}
	return privateExamResponse(exam), nil
}

func (s *server) ResetAssignmentAttempt(ctx context.Context, req *examsClient.ResetAssignmentAttemptRequest) (*examsClient.ResetAssignmentAttemptResponse, error) {
	if err := s.service.ResetAssignmentAttempt(ctx, uint(req.AssignmentId), req.StudentId); err != nil {
		return nil, err
	}
	return &examsClient.ResetAssignmentAttemptResponse{Reset_: true}, nil
}

func (s *server) ReconcileAssignmentSubmissions(ctx context.Context, req *examsClient.ReconcileAssignmentSubmissionsRequest) (*examsClient.ReconcileAssignmentSubmissionsResponse, error) {
	count, err := s.service.ReconcileAssignmentSubmissions(ctx, uint(req.AssignmentId))
	if err != nil {
		return nil, err
	}
	return &examsClient.ReconcileAssignmentSubmissionsResponse{EnqueuedCount: int32(count)}, nil
}
