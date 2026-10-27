package dto

import (
	"testing"

	"ego/services/learning/internal/model"
)

func TestGetExamsQueryNormalize(t *testing.T) {
	tests := []struct {
		name string
		in   model.ExamType
		want model.ExamType
	}{
		{"default THPT", "", model.ExamTypeTHPT},
		{"trim and upper", " toeic ", model.ExamTypeTOEIC},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := GetExamsQuery{Type: tt.in}
			query.Normalize()
			if query.Type != tt.want {
				t.Fatalf("type = %q, want %q", query.Type, tt.want)
			}
		})
	}
}

func TestToGetExamResponseSummarizesSectionsAndParts(t *testing.T) {
	part1, part5 := model.PartCode("1"), model.PartCode("5")
	listening, reading := 45, 75
	exam := &model.Exam{Type: model.ExamTypeTOEIC, Sections: []model.Section{
		{Code: model.SectionCode("READING"), Order: 2, Duration: &reading, Questions: []model.Question{{Part: &part5}, {Part: &part5}}},
		{Code: model.SectionCode("LISTENING"), Order: 1, Duration: &listening, Groups: []model.Group{{Questions: []model.Question{{Part: &part1}}}}},
	}}

	resp := ToGetExamResponse(exam)
	if resp.Duration != 120 || resp.TotalSections != 2 || resp.TotalQuestions != 3 {
		t.Fatalf("totals = duration %d sections %d questions %d", resp.Duration, resp.TotalSections, resp.TotalQuestions)
	}
	if resp.Sections[0].Code != "LISTENING" || resp.Sections[0].QuestionCount != 1 || resp.Sections[1].QuestionCount != 2 {
		t.Fatalf("sections not sorted by order: %#v %#v", resp.Sections[0], resp.Sections[1])
	}
	if len(resp.Parts) != 2 || resp.Parts[0].Part != "PART 1" || resp.Parts[0].SectionCode != "LISTENING" || resp.Parts[1].QuestionCount != 2 {
		t.Fatalf("unexpected parts: %#v %#v", resp.Parts[0], resp.Parts[1])
	}
}
