package service

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"ego/services/knowledge/internal/model"
	"ego/services/knowledge/internal/repository"

	"gorm.io/gorm"
)

type fakeTopicRepository struct {
	repository.Repository
	topics      []*model.Topic
	topic       *model.Topic
	sections    []*model.Section
	lessons     []*model.Lesson
	lesson      *model.Lesson
	topicErr    error
	sectionsErr error
	lessonErr   error
}

func (f *fakeTopicRepository) GetTopics(context.Context) ([]*model.Topic, error) {
	return f.topics, f.topicErr
}

func (f *fakeTopicRepository) GetTopicByID(context.Context, uint) (*model.Topic, error) {
	return f.topic, f.topicErr
}

func (f *fakeTopicRepository) GetSectionsByTopic(context.Context, uint) ([]*model.Section, error) {
	return f.sections, f.sectionsErr
}

func (f *fakeTopicRepository) GetLessonsBySection(context.Context, uint) ([]*model.Lesson, error) {
	return f.lessons, f.lessonErr
}

func (f *fakeTopicRepository) GetLessonByID(context.Context, uint) (*model.Lesson, error) {
	return f.lesson, f.lessonErr
}

var errDB = errors.New("db down")

func TestGetTopics(t *testing.T) {
	topic := &model.Topic{Name: "Daily", IsAudio: true, Url: "u"}
	topic.ID = 3

	tests := []struct {
		name       string
		repo       *fakeTopicRepository
		wantStatus int
		wantLen    int
	}{
		{name: "maps topics", repo: &fakeTopicRepository{topics: []*model.Topic{topic}}, wantLen: 1},
		{name: "empty list is not nil", repo: &fakeTopicRepository{}, wantLen: 0},
		{name: "db error is 500", repo: &fakeTopicRepository{topicErr: errDB}, wantStatus: http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, httpErr := New(tt.repo).GetTopics(context.Background())
			if tt.wantStatus != 0 {
				if httpErr == nil || httpErr.StatusCode != tt.wantStatus {
					t.Fatalf("want status %d, got %v", tt.wantStatus, httpErr)
				}
				return
			}
			if httpErr != nil {
				t.Fatalf("unexpected error: %v", httpErr)
			}
			if resp.Topics == nil || len(resp.Topics) != tt.wantLen {
				t.Fatalf("want %d topics (non-nil), got %#v", tt.wantLen, resp.Topics)
			}
			if tt.wantLen == 1 {
				got := resp.Topics[0]
				if got.ID != 3 || got.Name != "Daily" || !got.IsAudio || got.Url != "u" {
					t.Fatalf("bad mapping: %#v", got)
				}
			}
		})
	}
}

func TestGetSectionsByTopic(t *testing.T) {
	section := &model.Section{Name: "Part 1", TopicID: 3}
	section.ID = 7

	tests := []struct {
		name       string
		repo       *fakeTopicRepository
		wantStatus int
		wantLen    int
	}{
		{name: "maps sections", repo: &fakeTopicRepository{topic: &model.Topic{}, sections: []*model.Section{section}}, wantLen: 1},
		{name: "existing topic without sections is 200 and empty", repo: &fakeTopicRepository{topic: &model.Topic{}}, wantLen: 0},
		{name: "missing topic is 404", repo: &fakeTopicRepository{topicErr: gorm.ErrRecordNotFound}, wantStatus: http.StatusNotFound},
		{name: "topic lookup error is 500", repo: &fakeTopicRepository{topicErr: errDB}, wantStatus: http.StatusInternalServerError},
		{name: "sections error is 500", repo: &fakeTopicRepository{topic: &model.Topic{}, sectionsErr: errDB}, wantStatus: http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, httpErr := New(tt.repo).GetSectionsByTopic(context.Background(), 3)
			if tt.wantStatus != 0 {
				if httpErr == nil || httpErr.StatusCode != tt.wantStatus {
					t.Fatalf("want status %d, got %v", tt.wantStatus, httpErr)
				}
				return
			}
			if httpErr != nil {
				t.Fatalf("unexpected error: %v", httpErr)
			}
			if resp.Sections == nil || len(resp.Sections) != tt.wantLen {
				t.Fatalf("want %d sections (non-nil), got %#v", tt.wantLen, resp.Sections)
			}
			if tt.wantLen == 1 {
				got := resp.Sections[0]
				if got.ID != 7 || got.Name != "Part 1" || got.TopicID != 3 {
					t.Fatalf("bad mapping: %#v", got)
				}
			}
		})
	}
}

func TestGetLessonsBySection(t *testing.T) {
	lesson := &model.Lesson{Title: "L1", SectionID: 7}
	lesson.ID = 9

	resp, httpErr := New(&fakeTopicRepository{lessons: []*model.Lesson{lesson}}).GetLessonsBySection(context.Background(), 7)
	if httpErr != nil {
		t.Fatalf("unexpected error: %v", httpErr)
	}
	got := resp.Lessons[0]
	if got.ID != 9 || got.Title != "L1" || got.SectionID != 7 || got.Transcripts == nil || len(got.Transcripts) != 0 {
		t.Fatalf("bad mapping (transcripts must be empty, non-nil): %#v", got)
	}

	if _, httpErr := New(&fakeTopicRepository{lessonErr: errDB}).GetLessonsBySection(context.Background(), 7); httpErr == nil || httpErr.StatusCode != http.StatusInternalServerError {
		t.Fatalf("want 500, got %v", httpErr)
	}
}

func TestGetLessonByID(t *testing.T) {
	transcript := model.Transcript{Content: "hi", Order: 1, TimeStart: 0.5, TimeEnd: 1.5, Url: "a.mp3", LessonID: 9}
	transcript.ID = 11
	lesson := &model.Lesson{Title: "L1", Url: "v.mp4", SectionID: 7, Transcripts: []model.Transcript{transcript}}
	lesson.ID = 9

	tests := []struct {
		name       string
		repo       *fakeTopicRepository
		wantStatus int
	}{
		{name: "maps lesson with transcripts", repo: &fakeTopicRepository{lesson: lesson}},
		{name: "missing lesson is 404", repo: &fakeTopicRepository{lessonErr: gorm.ErrRecordNotFound}, wantStatus: http.StatusNotFound},
		{name: "db error is 500", repo: &fakeTopicRepository{lessonErr: errDB}, wantStatus: http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, httpErr := New(tt.repo).GetLessonByID(context.Background(), 9)
			if tt.wantStatus != 0 {
				if httpErr == nil || httpErr.StatusCode != tt.wantStatus {
					t.Fatalf("want status %d, got %v", tt.wantStatus, httpErr)
				}
				return
			}
			if httpErr != nil {
				t.Fatalf("unexpected error: %v", httpErr)
			}
			got := resp.Lesson
			if got.ID != 9 || got.Url != "v.mp4" || len(got.Transcripts) != 1 {
				t.Fatalf("bad lesson mapping: %#v", got)
			}
			tr := got.Transcripts[0]
			if tr.ID != 11 || tr.Content != "hi" || tr.Order != 1 || tr.TimeStart != 0.5 || tr.TimeEnd != 1.5 || tr.Url != "a.mp3" || tr.LessonID != 9 {
				t.Fatalf("bad transcript mapping: %#v", tr)
			}
		})
	}
}
