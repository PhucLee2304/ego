package dto

type Topic struct {
	ID      uint   `json:"id"`
	Name    string `json:"name"`
	IsAudio bool   `json:"isAudio"`
	Url     string `json:"url"`
}

type GetTopicsResponse struct {
	Topics []*Topic `json:"topics"`
}

type Section struct {
	ID      uint   `json:"id"`
	Name    string `json:"name"`
	TopicID uint   `json:"topicId"`
}

type GetSectionsResponse struct {
	Sections []*Section `json:"sections"`
}

type Transcript struct {
	ID        uint    `json:"id"`
	Content   string  `json:"content"`
	Order     uint    `json:"order"`
	TimeStart float64 `json:"timeStart"`
	TimeEnd   float64 `json:"timeEnd"`
	Url       string  `json:"url"`
	LessonID  uint    `json:"lessonId"`
}

type Lesson struct {
	ID          uint          `json:"id"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Subtitle    string        `json:"subtitle"`
	Url         string        `json:"url"`
	SectionID   uint          `json:"sectionId"`
	Transcripts []*Transcript `json:"transcripts"`
}

type GetLessonsResponse struct {
	Lessons []*Lesson `json:"lessons"`
}

type GetLessonByIDResponse struct {
	Lesson *Lesson `json:"lesson"`
}
