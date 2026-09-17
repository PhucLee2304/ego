package dto

import (
	"testing"

	"ego/services/exams/internal/model"
	"gorm.io/gorm"
)

func TestSubmitAttemptRequestValidate(t *testing.T) {
	optionID := uint(10)

	tests := []struct {
		name    string
		request SubmitAttemptRequest
		wantErr bool
	}{
		{
			name:    "empty answers are valid",
			request: SubmitAttemptRequest{Answers: []SubmitAttemptAnswer{}},
		},
		{
			name: "valid answer",
			request: SubmitAttemptRequest{Answers: []SubmitAttemptAnswer{
				{QuestionID: 1, SelectedOptionID: &optionID},
			}},
		},
		{
			name: "duplicate question",
			request: SubmitAttemptRequest{Answers: []SubmitAttemptAnswer{
				{QuestionID: 1, SelectedOptionID: &optionID},
				{QuestionID: 1, SelectedOptionID: nil},
			}},
			wantErr: true,
		},
		{
			name: "zero question",
			request: SubmitAttemptRequest{Answers: []SubmitAttemptAnswer{
				{QuestionID: 0, SelectedOptionID: &optionID},
			}},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.request.Validate()
			if (err != nil) != test.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestCreateExamAttemptRequestValidateContext(t *testing.T) {
	contextID := uint(100)

	tests := []struct {
		name    string
		request CreateExamAttemptRequest
		wantErr bool
	}{
		{
			name: "direct test attempt",
			request: CreateExamAttemptRequest{
				Mode:        model.AttemptModeTest,
				ContextType: model.AttemptContextTypeStandalone,
			},
		},
		{
			name: "standalone with context id",
			request: CreateExamAttemptRequest{
				Mode:        model.AttemptModeTest,
				ContextType: model.AttemptContextTypeStandalone,
				ContextID:   &contextID,
			},
			wantErr: true,
		},
		{
			name: "classroom assignment test attempt",
			request: CreateExamAttemptRequest{
				Mode:        model.AttemptModeTest,
				ContextType: model.AttemptContextTypeClassroomAssignment,
				ContextID:   &contextID,
			},
		},
		{
			name: "classroom assignment requires context id",
			request: CreateExamAttemptRequest{
				Mode:        model.AttemptModeTest,
				ContextType: model.AttemptContextTypeClassroomAssignment,
			},
			wantErr: true,
		},
		{
			name: "classroom assignment must be test",
			request: CreateExamAttemptRequest{
				Mode:        model.AttemptModePractice,
				ContextType: model.AttemptContextTypeClassroomAssignment,
				ContextID:   &contextID,
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.request.Validate()
			if (err != nil) != test.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestAttemptQuestionResponseVisibility(t *testing.T) {
	correctOptionID := uint(11)
	isCorrect := true
	attempt := &model.Attempt{
		Status:         model.AttemptStatusInProgress,
		TotalQuestions: 1,
		Exam: model.Exam{
			Sections: []model.Section{
				{
					Code: model.SectionCodeListening,
					Groups: []model.Group{
						{
							Transcript:  "secret transcript",
							Explanation: "group explanation",
							Questions: []model.Question{
								{
									Model:       modelWithID(1),
									Content:     "question",
									Explanation: "question explanation",
									Options: []model.Option{
										{Model: modelWithID(correctOptionID), IsCorrect: true},
										{Model: modelWithID(12), IsCorrect: false},
									},
								},
							},
						},
					},
					Questions: []model.Question{
						{
							Model:       modelWithID(1),
							GroupID:     uintPointer(1),
							Content:     "question",
							Explanation: "question explanation",
							Options: []model.Option{
								{Model: modelWithID(correctOptionID), IsCorrect: true},
								{Model: modelWithID(12), IsCorrect: false},
							},
						},
					},
				},
			},
		},
		Answers: []model.AttemptAnswer{
			{QuestionID: 1, SelectedOptionID: &correctOptionID, IsCorrect: &isCorrect},
		},
	}

	active := ToAttemptResponseWithQuestions(attempt)
	activeGroup := active.Sections[0].Groups[0]
	activeQuestion := activeGroup.Questions[0]
	if activeGroup.Transcript != "" || activeGroup.Explanation != "" || activeQuestion.Explanation != "" {
		t.Fatal("active response leaked transcript or explanation")
	}
	if activeQuestion.CorrectOptionID != nil || activeQuestion.IsCorrect != nil {
		t.Fatal("active response leaked grading result")
	}

	attempt.Status = model.AttemptStatusSubmitted
	submitted := ToAttemptHistoryResponse(attempt)
	submittedGroup := submitted.Sections[0].Groups[0]
	submittedQuestion := submittedGroup.Questions[0]
	if submittedGroup.Transcript == "" || submittedQuestion.Explanation == "" {
		t.Fatal("submitted history omitted review content")
	}
	if submittedQuestion.CorrectOptionID == nil || *submittedQuestion.CorrectOptionID != correctOptionID {
		t.Fatal("submitted history omitted the correct option")
	}
	if submittedQuestion.IsCorrect == nil || !*submittedQuestion.IsCorrect {
		t.Fatal("submitted history omitted the answer result")
	}

	attempt.Status = model.AttemptStatusCancelled
	cancelled := ToAttemptHistoryResponse(attempt)
	cancelledQuestion := cancelled.Sections[0].Groups[0].Questions[0]
	if cancelledQuestion.CorrectOptionID != nil || cancelledQuestion.IsCorrect != nil {
		t.Fatal("cancelled history leaked grading result")
	}
}

func uintPointer(value uint) *uint {
	return &value
}

func modelWithID(id uint) gorm.Model {
	return gorm.Model{ID: id}
}
