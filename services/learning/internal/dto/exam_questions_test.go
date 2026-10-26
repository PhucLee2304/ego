package dto

import (
	"testing"

	"ego/services/learning/internal/model"

	"gorm.io/gorm"
)

func TestToGetExamQuestionsResponseSortsGroupsByPartThenOrder(t *testing.T) {
	part := func(code model.PartCode) *model.PartCode { return &code }
	group := func(id uint, groupPart *model.PartCode, order int, questionPart *model.PartCode) model.Group {
		return model.Group{
			Model: gorm.Model{ID: id},
			Part:  groupPart,
			Order: order,
			Questions: []model.Question{
				{Model: gorm.Model{ID: id * 10}, Part: questionPart, Order: 1},
			},
		}
	}

	tests := []struct {
		name   string
		groups []model.Group
		want   []uint
	}{
		{
			name:   "group part wins over order",
			groups: []model.Group{group(1, part(model.Part2), 1, part(model.Part2)), group(2, part(model.Part1), 2, part(model.Part1))},
			want:   []uint{2, 1},
		},
		{
			name:   "same part sorted by order",
			groups: []model.Group{group(1, part(model.Part3), 5, part(model.Part3)), group(2, part(model.Part3), 1, part(model.Part3))},
			want:   []uint{2, 1},
		},
		{
			name:   "fallback to question part when group part missing",
			groups: []model.Group{group(1, nil, 1, part(model.Part4)), group(2, nil, 9, part(model.Part3))},
			want:   []uint{2, 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exam := &model.Exam{Sections: []model.Section{{Code: model.SectionCodeListening, Groups: tt.groups}}}
			got := ToGetExamQuestionsResponse(exam, GetExamQuestionsQuery{})
			if len(got.Sections) != 1 || len(got.Sections[0].Groups) != len(tt.want) {
				t.Fatalf("unexpected sections: %+v", got.Sections)
			}
			for i, id := range tt.want {
				if got.Sections[0].Groups[i].ID != id {
					t.Fatalf("group[%d] = %d, want %d", i, got.Sections[0].Groups[i].ID, id)
				}
			}
		})
	}
}
