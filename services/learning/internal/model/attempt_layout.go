package model

// AttemptPage là một trang của attempt, chụp lại lúc tạo attempt vào Attempt.Layout.
type AttemptPage struct {
	Index       int         `json:"index"`
	Section     SectionCode `json:"section"`
	Part        *PartCode   `json:"part,omitempty"`
	GroupID     *uint       `json:"groupId,omitempty"`
	QuestionIDs []uint      `json:"questionIds"`
	AudioMs     int         `json:"audioMs"`
	GapMs       int         `json:"gapMs"`
}
