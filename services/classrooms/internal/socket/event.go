package socket

import "time"

const EventGradebookUpdated = "GRADEBOOK_UPDATED"

type GradebookUpdatedEvent struct {
	Type    string                  `json:"type"`
	Payload GradebookUpdatedPayload `json:"payload"`
}

type GradebookUpdatedPayload struct {
	ClassroomID  uint      `json:"classroomId"`
	AssignmentID uint      `json:"assignmentId"`
	StudentID    string    `json:"studentId"`
	AttemptID    uint      `json:"attemptId"`
	Status       string    `json:"status"`
	Score        *float64  `json:"score"`
	UpdatedAt    time.Time `json:"updatedAt"`
}
