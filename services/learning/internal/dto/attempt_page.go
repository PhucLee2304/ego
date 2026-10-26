package dto

import (
	"time"

	"ego/services/learning/internal/model"
)

// AttemptNavigatorResponse vẽ bảng câu hỏi; không chứa nội dung câu (PLAN_TOEIC_ATTEMPT.md mục 1.5).
type AttemptNavigatorResponse struct {
	Flow           string                  `json:"flow"`
	Stage          *string                 `json:"stage,omitempty"`
	CurrentPage    *int                    `json:"currentPage,omitempty"`
	PageStartedAt  *time.Time              `json:"pageStartedAt,omitempty"`
	ServerTime     time.Time               `json:"serverTime"`
	ExpiresAt      *time.Time              `json:"expiresAt,omitempty"`
	TotalQuestions int                     `json:"totalQuestions"`
	AnsweredCount  int                     `json:"answeredCount"`
	Pages          []*AttemptNavigatorPage `json:"pages"`
}

type AttemptNavigatorPage struct {
	Index     int                         `json:"index"`
	Section   string                      `json:"section"`
	Part      *string                     `json:"part,omitempty"`
	Questions []*AttemptNavigatorQuestion `json:"questions"`
}

type AttemptNavigatorQuestion struct {
	ID       uint `json:"id"`
	Number   int  `json:"number"`
	Answered bool `json:"answered"`
	Locked   bool `json:"locked"`
}

// AttemptPageResponse là nội dung một trang của attempt.
type AttemptPageResponse struct {
	Index        int                    `json:"index"`
	Section      string                 `json:"section"`
	Part         *string                `json:"part,omitempty"`
	Group        *AttemptPageGroup      `json:"group,omitempty"`
	Questions    []*AttemptPageQuestion `json:"questions"`
	Listening    *AttemptPageListening  `json:"listening,omitempty"`
	NextAudioURL *string                `json:"nextAudioUrl,omitempty"`
}

type AttemptPageGroup struct {
	Title           string  `json:"title"`
	Instruction     string  `json:"instruction"`
	ImageURL        *string `json:"imageUrl"`
	AudioURL        *string `json:"audioUrl"`
	AudioDurationMs *int    `json:"audioDurationMs"`
}

type AttemptPageQuestion struct {
	ID               uint                 `json:"id"`
	Number           int                  `json:"number"`
	Content          string               `json:"content"`
	Options          []*AttemptPageOption `json:"options"`
	SelectedOptionID *uint                `json:"selectedOptionId,omitempty"`
}

type AttemptPageOption struct {
	ID      uint    `json:"id"`
	Key     string  `json:"key"`
	Content *string `json:"content"`
}

// AttemptPageListening: client dùng elapsedMs để phát tiếp đúng chỗ sau F5;
// minAdvanceAt là mốc sớm nhất server nhận ATTEMPT_LISTENING_ADVANCE.
type AttemptPageListening struct {
	ElapsedMs    int64     `json:"elapsedMs"`
	GapMs        int       `json:"gapMs"`
	MinAdvanceAt time.Time `json:"minAdvanceAt"`
}

// QuestionNumbers đánh số câu theo layout: Question.Number nếu có, không thì số thứ tự toàn bài.
func QuestionNumbers(layout []model.AttemptPage, numbers map[uint]int) map[uint]int {
	result := make(map[uint]int)
	position := 0
	for _, page := range layout {
		for _, id := range page.QuestionIDs {
			position++
			if number, ok := numbers[id]; ok {
				result[id] = number
			} else {
				result[id] = position
			}
		}
	}
	return result
}

// ToAttemptPageQuestions map câu của trang. hideContent ẩn nội dung câu và option
// (Part 1/2 khi đang làm, như đề thật); transcript/explanation không bao giờ có ở đây.
func ToAttemptPageQuestions(questions []*model.Question, numbers map[uint]int, answers map[uint]*uint, hideContent bool) []*AttemptPageQuestion {
	result := make([]*AttemptPageQuestion, 0, len(questions))
	for _, question := range questions {
		item := &AttemptPageQuestion{
			ID:               question.ID,
			Number:           numbers[question.ID],
			Content:          question.Content,
			Options:          make([]*AttemptPageOption, 0, len(question.Options)),
			SelectedOptionID: answers[question.ID],
		}
		if hideContent {
			item.Content = ""
		}
		for _, option := range question.Options {
			content := option.Content
			if hideContent {
				content = nil
			}
			item.Options = append(item.Options, &AttemptPageOption{ID: option.ID, Key: string(option.Key), Content: content})
		}
		result = append(result, item)
	}
	return result
}

func ToAttemptPageGroup(group *model.Group) *AttemptPageGroup {
	if group == nil {
		return nil
	}
	return &AttemptPageGroup{
		Title:           group.Title,
		Instruction:     group.Instruction,
		ImageURL:        group.ImageURL,
		AudioURL:        group.AudioURL,
		AudioDurationMs: group.AudioDurationMs,
	}
}

func PartString(part *model.PartCode) *string {
	if part == nil {
		return nil
	}
	value := string(*part)
	return &value
}

// ListeningStateResponse là payload ACK của ATTEMPT_LISTENING_ADVANCE.
type ListeningStateResponse struct {
	Stage         string     `json:"stage"`
	CurrentPage   *int       `json:"currentPage,omitempty"`
	PageStartedAt *time.Time `json:"pageStartedAt,omitempty"`
	ServerTime    time.Time  `json:"serverTime"`
}
