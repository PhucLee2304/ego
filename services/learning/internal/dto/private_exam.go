package dto

// Đề private (type ASSIGNMENT) chỉ được tạo/sửa nội bộ từ phía assignment,
// không có route HTTP riêng.

type CreatePrivateExamRequest struct {
	Title       string                             `validate:"required,notblank"`
	Description *string                            `validate:"omitempty"`
	Questions   []CreatePrivateExamQuestionRequest `validate:"required,min=1,dive"`
}

type UpdatePrivateExamRequest struct {
	Title          string
	Description    *string
	Questions      []UpdatePrivateExamQuestionRequest
	CorrectionOnly bool
	ActorID        string
	Reason         string
	Changes        []string
}

type UpdatePrivateExamQuestionRequest struct {
	ID                 uint
	Content            string
	Explanation        *string
	Options            []string
	CorrectOptionIndex int
	ExcludedFromScore  bool
	OptionIDs          []uint
}

type CreatePrivateExamQuestionRequest struct {
	Content            string   `validate:"required,notblank"`
	Explanation        *string  `validate:"omitempty"`
	Options            []string `validate:"required,len=4,dive,notblank"`
	CorrectOptionIndex int      `validate:"gte=0,lte=3"`
	ExcludedFromScore  bool
}
