package service

import (
	"reflect"
	"testing"

	learningConfig "ego/services/learning/config"
	"ego/services/learning/internal/model"

	"gorm.io/gorm"
)

// layoutTestExam dựng đề TOEIC rút gọn: mỗi part 2 câu (Part 3/4/6/7: 1 group 2 câu).
// ID câu = part*10 + thứ tự; ID group = part*100 + thứ tự. Group Part 2 đặt order nhỏ hơn
// Part 1 để kiểm tra sắp theo part trước.
func layoutTestExam() *model.Exam {
	duration, tail := 20000, 2000
	part := func(code model.PartCode) *model.PartCode { return &code }
	question := func(sectionID uint, p model.PartCode, id uint, order int) model.Question {
		return model.Question{Model: gorm.Model{ID: id}, Part: part(p), Order: order, SectionID: sectionID}
	}
	group := func(sectionID uint, p model.PartCode, id uint, order int, questions ...model.Question) model.Group {
		g := model.Group{Model: gorm.Model{ID: id}, Part: part(p), Order: order, SectionID: sectionID, Questions: questions}
		if sectionID == 1 {
			g.AudioDurationMs, g.TrailingSilenceMs = &duration, &tail
		}
		return g
	}

	return &model.Exam{
		Type: model.ExamTypeTOEIC,
		Sections: []model.Section{
			{
				Model: gorm.Model{ID: 1}, Code: model.SectionCodeListening, Order: 1,
				Groups: []model.Group{
					group(1, model.Part2, 201, 1, question(1, model.Part2, 21, 3)),
					group(1, model.Part2, 202, 2, question(1, model.Part2, 22, 4)),
					group(1, model.Part1, 101, 3, question(1, model.Part1, 11, 1)),
					group(1, model.Part1, 102, 4, question(1, model.Part1, 12, 2)),
					group(1, model.Part3, 301, 5, question(1, model.Part3, 31, 5), question(1, model.Part3, 32, 6)),
				},
			},
			{
				Model: gorm.Model{ID: 2}, Code: model.SectionCodeReading, Order: 2,
				Groups: []model.Group{
					group(2, model.Part5, 501, 7, question(2, model.Part5, 51, 1)),
					group(2, model.Part5, 502, 8, question(2, model.Part5, 52, 2)),
					group(2, model.Part7, 701, 9, question(2, model.Part7, 71, 3), question(2, model.Part7, 72, 4)),
				},
			},
		},
	}
}

func TestBuildAttemptLayout(t *testing.T) {
	gap := listeningGapConfig{5000, 5000, 8000, 8000}
	reading := model.SectionCodeReading

	tests := []struct {
		name    string
		section *model.SectionCode
		parts   []model.PartCode
		want    [][]uint // questionIds theo từng trang
	}{
		{name: "full", want: [][]uint{{11}, {12}, {21}, {22}, {31, 32}, {51}, {52}, {71, 72}}},
		{name: "only part 2", parts: []model.PartCode{model.Part2}, want: [][]uint{{21}, {22}}},
		{name: "part 3 + 5", parts: []model.PartCode{model.Part3, model.Part5}, want: [][]uint{{31, 32}, {51}, {52}}},
		{name: "only reading", section: &reading, want: [][]uint{{51}, {52}, {71, 72}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pages := buildAttemptLayout(layoutTestExam(), tt.section, tt.parts, gap)
			got := make([][]uint, len(pages))
			for i, page := range pages {
				if page.Index != i {
					t.Fatalf("page %d has index %d", i, page.Index)
				}
				got[i] = page.QuestionIDs
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("pages = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildAttemptLayoutTiming(t *testing.T) {
	pages := buildAttemptLayout(layoutTestExam(), nil, nil, listeningGapConfig{5000, 5000, 8000, 8000})
	tests := []struct {
		name    string
		page    int
		section model.SectionCode
		audioMs int
		gapMs   int
	}{
		{name: "part 1 tops up trailing silence to 5s", page: 0, section: model.SectionCodeListening, audioMs: 20000, gapMs: 3000},
		{name: "part 3 tops up to 8s", page: 4, section: model.SectionCodeListening, audioMs: 20000, gapMs: 6000},
		{name: "reading page has no timing", page: 5, section: model.SectionCodeReading, audioMs: 0, gapMs: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page := pages[tt.page]
			if page.Section != tt.section || page.AudioMs != tt.audioMs || page.GapMs != tt.gapMs {
				t.Fatalf("page = %+v, want section %s audio %d gap %d", page, tt.section, tt.audioMs, tt.gapMs)
			}
			if page.GroupID == nil {
				t.Fatal("expected group id")
			}
		})
	}
}

func TestListeningTiming(t *testing.T) {
	part := func(code model.PartCode) *model.PartCode { return &code }
	ms := func(v int) *int { return &v }
	gap := listeningGapConfig{5000, 5000, 8000, 8000}
	tests := []struct {
		name      string
		part      *model.PartCode
		group     *model.Group
		wantAudio int
		wantGap   int
	}{
		{name: "missing trailing silence uses full target", part: part(model.Part2), group: &model.Group{AudioDurationMs: ms(18000)}, wantAudio: 18000, wantGap: 5000},
		{name: "silence longer than target", part: part(model.Part4), group: &model.Group{AudioDurationMs: ms(70000), TrailingSilenceMs: ms(11000)}, wantAudio: 70000, wantGap: 0},
		{name: "no group", part: part(model.Part1), wantAudio: 0, wantGap: 5000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			audio, gotGap := listeningTiming(tt.part, tt.group, gap)
			if audio != tt.wantAudio || gotGap != tt.wantGap {
				t.Fatalf("listeningTiming = (%d, %d), want (%d, %d)", audio, gotGap, tt.wantAudio, tt.wantGap)
			}
		})
	}
}

func TestBuildAttemptLayoutUngroupedQuestions(t *testing.T) {
	exam := &model.Exam{
		Type: model.ExamTypeTHPT,
		Sections: []model.Section{{
			Code: model.SectionCodeFull, Order: 1,
			Questions: []model.Question{
				{Model: gorm.Model{ID: 2}, Order: 2},
				{Model: gorm.Model{ID: 1}, Order: 1},
			},
		}},
	}
	pages := buildAttemptLayout(exam, nil, nil, defaultListeningGap)
	if len(pages) != 2 || pages[0].QuestionIDs[0] != 1 || pages[1].QuestionIDs[0] != 2 || pages[0].GroupID != nil {
		t.Fatalf("pages = %+v", pages)
	}
}

func TestAttemptFlowFor(t *testing.T) {
	enabled := &learningConfig.AppConfig{TOEICStrictFlowEnabled: true}
	toeic := &model.Exam{Type: model.ExamTypeTOEIC}
	thpt := &model.Exam{Type: model.ExamTypeTHPT}
	tests := []struct {
		name string
		cfg  *learningConfig.AppConfig
		exam *model.Exam
		mode model.AttemptMode
		want model.AttemptFlow
	}{
		{name: "toeic test strict", cfg: enabled, exam: toeic, mode: model.AttemptModeTest, want: model.AttemptFlowTOEICStrict},
		{name: "toeic practice strict", cfg: enabled, exam: toeic, mode: model.AttemptModePractice, want: model.AttemptFlowTOEICStrict},
		{name: "flag off", cfg: &learningConfig.AppConfig{}, exam: toeic, mode: model.AttemptModeTest, want: model.AttemptFlowFree},
		{name: "nil config", exam: toeic, mode: model.AttemptModeTest, want: model.AttemptFlowFree},
		{name: "thpt free", cfg: enabled, exam: thpt, mode: model.AttemptModeTest, want: model.AttemptFlowFree},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := (&service{cfg: tt.cfg}).attemptFlowFor(tt.exam, tt.mode); got != tt.want {
				t.Fatalf("attemptFlowFor = %s, want %s", got, tt.want)
			}
		})
	}
}
