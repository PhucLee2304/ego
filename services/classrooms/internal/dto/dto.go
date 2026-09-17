package dto

import "ego/services/classrooms/internal/model"

type ValidateAssignmentAttemptResponse struct {
	AssignmentID uint `json:"assignmentId"`
	ClassroomID  uint `json:"classroomId"`
	ExamID       uint `json:"examId"`
}

type AssignmentSubmissionResponse struct {
	AssignmentID uint     `json:"assignmentId"`
	StudentID    string   `json:"studentId"`
	AttemptID    uint     `json:"attemptId"`
	Score        *float64 `json:"score,omitempty"`
}

type AssignmentSubmissionSyncStatusResponse struct {
	Exists         bool `json:"exists"`
	AttemptMatched bool `json:"attemptMatched"`
	ScoreMatched   bool `json:"scoreMatched"`
	Synced         bool `json:"synced"`
}

func ToAssignmentSubmissionResponse(submission *model.AssignmentSubmission) *AssignmentSubmissionResponse {
	return &AssignmentSubmissionResponse{
		AssignmentID: submission.AssignmentID,
		StudentID:    submission.StudentID,
		AttemptID:    submission.AttemptID,
		Score:        submission.Score,
	}
}
