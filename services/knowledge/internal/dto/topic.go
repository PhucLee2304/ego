package dto

import "ego/services/knowledge/internal/model"

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

func ToTopic(topic *model.Topic) *Topic {
	return &Topic{
		ID:      topic.ID,
		Name:    topic.Name,
		IsAudio: topic.IsAudio,
		Url:     topic.Url,
	}
}

func ToSection(section *model.Section) *Section {
	return &Section{
		ID:      section.ID,
		Name:    section.Name,
		TopicID: section.TopicID,
	}
}

func ToTranscript(transcript *model.Transcript) *Transcript {
	return &Transcript{
		ID:        transcript.ID,
		Content:   transcript.Content,
		Order:     transcript.Order,
		TimeStart: transcript.TimeStart,
		TimeEnd:   transcript.TimeEnd,
		Url:       transcript.Url,
		LessonID:  transcript.LessonID,
	}
}

// ToLesson map lesson kèm transcript đã preload. Lesson không preload transcript
// (danh sách lesson của một section) cho ra `transcripts: []`.
func ToLesson(lesson *model.Lesson) *Lesson {
	transcripts := make([]*Transcript, len(lesson.Transcripts))
	for i := range lesson.Transcripts {
		transcripts[i] = ToTranscript(&lesson.Transcripts[i])
	}

	return &Lesson{
		ID:          lesson.ID,
		Title:       lesson.Title,
		Description: lesson.Description,
		Subtitle:    lesson.Subtitle,
		Url:         lesson.Url,
		SectionID:   lesson.SectionID,
		Transcripts: transcripts,
	}
}
