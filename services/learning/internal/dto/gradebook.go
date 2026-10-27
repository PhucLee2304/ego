package dto

import (
	"time"

	"ego/services/learning/internal/model"
)

type GradebookRangeQuery struct {
	From *time.Time `schema:"from"`
	To   *time.Time `schema:"to"`
}

type ClassGradebookResponse struct {
	ClassroomID    uint                       `json:"classroomId"`
	Assignments    []*GradebookAssignment     `json:"assignments"`
	Students       []*StudentProgressResponse `json:"students"`
	AverageScore   *float64                   `json:"averageScore"`
	CompletionRate float64                    `json:"completionRate"`
}

type GradebookAssignment struct {
	ID    uint      `json:"id"`
	Title string    `json:"title"`
	DueAt time.Time `json:"dueAt"`
}

type StudentProgressResponse struct {
	Student        *UserInfo                    `json:"student"`
	Assignments    []*StudentAssignmentProgress `json:"assignments"`
	AverageScore   *float64                     `json:"averageScore"`
	CompletionRate float64                      `json:"completionRate"`
}

type StudentAssignmentProgress struct {
	AssignmentID uint                   `json:"assignmentId"`
	Title        string                 `json:"title"`
	DueAt        time.Time              `json:"dueAt"`
	Status       model.SubmissionStatus `json:"status"`
	Score        *float64               `json:"score"`
}

type AssignmentAnalyticsResponse struct {
	AssignmentID   uint                         `json:"assignmentId"`
	TotalStudents  int                          `json:"totalStudents"`
	NotStarted     int                          `json:"notStarted"`
	InProgress     int                          `json:"inProgress"`
	Submitted      int                          `json:"submitted"`
	Late           int                          `json:"late"`
	AverageScore   *float64                     `json:"averageScore"`
	MinimumScore   *float64                     `json:"minimumScore"`
	MaximumScore   *float64                     `json:"maximumScore"`
	CompletionRate float64                      `json:"completionRate"`
	Questions      []*QuestionAnalyticsResponse `json:"questions"`
}

type QuestionAnalyticsResponse struct {
	QuestionID    uint    `json:"questionId"`
	Content       string  `json:"content"`
	Order         int     `json:"order"`
	AnsweredCount int64   `json:"answeredCount"`
	WrongCount    int64   `json:"wrongCount"`
	WrongRate     float64 `json:"wrongRate"`
}

