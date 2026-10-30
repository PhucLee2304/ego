package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/model"

	"gorm.io/datatypes"
)

// auditTimezone: giờ trong dòng lịch sử hiển thị theo giờ Việt Nam (người dùng của Ego).
var auditTimezone = time.FixedZone("ICT", 7*60*60)

func auditChanges(changes []string) datatypes.JSON {
	if len(changes) == 0 {
		return datatypes.JSON("[]")
	}
	data, err := json.Marshal(changes)
	if err != nil {
		return datatypes.JSON("[]")
	}
	return datatypes.JSON(data)
}

// describeContentChanges liệt kê thay đổi nội dung đề theo form sửa. Câu được ghép theo
// ID; câu không có ID là câu mới, ID cũ không còn trong form là câu bị xoá.
func describeContentChanges(exam *model.Exam, req dto.UpdateAssignmentRequest) []string {
	text := func(value *string) string {
		if value == nil {
			return ""
		}
		return strings.TrimSpace(*value)
	}
	var lines []string
	if title := strings.TrimSpace(req.Title); title != strings.TrimSpace(exam.Title) {
		lines = append(lines, fmt.Sprintf("Title: \"%s\" → \"%s\"", strings.TrimSpace(exam.Title), title))
	}
	if text(req.Description) != text(exam.Description) {
		lines = append(lines, "Description changed")
	}
	current := dto.ToEditableAssignmentQuestions(exam)
	oldIndex := make(map[uint]int, len(current))
	for i, question := range current {
		oldIndex[question.ID] = i
	}
	kept := make(map[uint]bool, len(req.Questions))
	for i, question := range req.Questions {
		label := fmt.Sprintf("Q%d", i+1)
		index, ok := oldIndex[question.ID]
		if question.ID == 0 || !ok {
			lines = append(lines, label+" added")
			continue
		}
		kept[question.ID] = true
		old := current[index]
		if index != i {
			lines = append(lines, fmt.Sprintf("%s moved from Q%d", label, index+1))
		}
		if strings.TrimSpace(question.Content) != strings.TrimSpace(old.Content) {
			lines = append(lines, label+": question text changed")
		}
		if optionsChanged(question.Options, old.Options) {
			lines = append(lines, label+": options changed")
		}
		if question.CorrectOptionIndex != old.CorrectOptionIndex {
			lines = append(lines, fmt.Sprintf("%s: correct answer %s → %s", label, optionLetter(old.CorrectOptionIndex), optionLetter(question.CorrectOptionIndex)))
		}
		if text(question.Explanation) != text(old.Explanation) {
			lines = append(lines, label+": explanation changed")
		}
		if question.ExcludedFromScore != old.ExcludedFromScore {
			if question.ExcludedFromScore {
				lines = append(lines, label+": excluded from scoring")
			} else {
				lines = append(lines, label+": counted in scoring again")
			}
		}
	}
	for i, question := range current {
		if !kept[question.ID] {
			lines = append(lines, fmt.Sprintf("Q%d removed (was \"%s\")", i+1, shorten(question.Content, 40)))
		}
	}
	return lines
}

// describeScheduleChanges liệt kê thay đổi lịch của một lớp đã có bài.
func describeScheduleChanges(className string, assignment *model.Assignment, schedule dto.AssignmentScheduleRequest) []string {
	var lines []string
	add := func(format string, args ...any) {
		lines = append(lines, className+": "+fmt.Sprintf(format, args...))
	}
	if !assignment.OpensAt.Equal(schedule.OpensAt) {
		add("opens %s → %s", auditTime(assignment.OpensAt), auditTime(schedule.OpensAt))
	}
	if !assignment.DueAt.Equal(schedule.DueAt) {
		add("due %s → %s", auditTime(assignment.DueAt), auditTime(schedule.DueAt))
	}
	if assignment.Duration != schedule.Duration {
		add("time limit %d → %d min", assignment.Duration, schedule.Duration)
	}
	if assignment.Proctored != schedule.Proctored {
		if schedule.Proctored {
			add("proctoring turned on")
		} else {
			add("proctoring turned off")
		}
	}
	if maxViolations := maxViolationsOrDefault(schedule.MaxViolations); schedule.Proctored && assignment.MaxViolations != maxViolations {
		add("max violations %d → %d", assignment.MaxViolations, maxViolations)
	}
	return lines
}

func describeAssigned(className string, schedule dto.AssignmentScheduleRequest) string {
	return fmt.Sprintf("Assigned to %s (opens %s, due %s, %d min)", className, auditTime(schedule.OpensAt), auditTime(schedule.DueAt), schedule.Duration)
}

func optionsChanged(next, old []string) bool {
	if len(next) != len(old) {
		return true
	}
	for i := range next {
		if strings.TrimSpace(next[i]) != strings.TrimSpace(old[i]) {
			return true
		}
	}
	return false
}

func optionLetter(index int) string {
	if index < 0 || index > 25 {
		return "?"
	}
	return string(rune('A' + index))
}

func auditTime(value time.Time) string {
	return value.In(auditTimezone).Format("Jan 2, 15:04")
}

func shorten(value string, limit int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit]) + "…"
}
