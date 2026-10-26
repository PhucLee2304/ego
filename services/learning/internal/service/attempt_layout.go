package service

import (
	"sort"

	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/model"
)

// listeningGapConfig là thời gian trả lời mục tiêu sau audio của trang nghe, theo part 1..4 (ms).
type listeningGapConfig [4]int

var defaultListeningGap = listeningGapConfig{5000, 5000, 8000, 8000}

func (s *service) listeningGap() listeningGapConfig {
	if s.cfg == nil {
		return defaultListeningGap
	}
	return s.cfg.TOEICListeningGapMs
}

// attemptFlowFor chốt flow lúc tạo attempt: mọi attempt TOEIC (TEST lẫn PRACTICE) là
// TOEIC_STRICT khi cờ TOEIC_STRICT_FLOW_ENABLED bật; còn lại là FREE.
func (s *service) attemptFlowFor(exam *model.Exam, mode model.AttemptMode) model.AttemptFlow {
	if s.cfg != nil && s.cfg.TOEICStrictFlowEnabled &&
		exam.Type == model.ExamTypeTOEIC &&
		(mode == model.AttemptModeTest || mode == model.AttemptModePractice) {
		return model.AttemptFlowTOEICStrict
	}
	return model.AttemptFlowFree
}

// buildAttemptLayout chia đề đã lọc theo section/parts thành các trang (PLAN_TOEIC_ATTEMPT.md mục 1.2).
// Thứ tự lấy từ dto.ToAttemptSections: section.order → part → group.order → question.order.
// Part 1, 2, 5 và câu không thuộc group: mỗi câu một trang; part khác: mỗi group một trang.
func buildAttemptLayout(exam *model.Exam, sectionCode *model.SectionCode, parts []model.PartCode, gap listeningGapConfig) []model.AttemptPage {
	groups := make(map[uint]*model.Group)
	for i := range exam.Sections {
		for j := range exam.Sections[i].Groups {
			group := &exam.Sections[i].Groups[j]
			groups[group.ID] = group
		}
	}

	pages := make([]model.AttemptPage, 0)
	add := func(section model.SectionCode, part *model.PartCode, group *model.Group, questionIDs []uint) {
		page := model.AttemptPage{
			Index:       len(pages),
			Section:     section,
			Part:        part,
			QuestionIDs: questionIDs,
		}
		if group != nil {
			groupID := group.ID
			page.GroupID = &groupID
		}
		if section == model.SectionCodeListening {
			page.AudioMs, page.GapMs = listeningTiming(part, group, gap)
		}
		pages = append(pages, page)
	}

	for _, section := range dto.ToAttemptSections(exam, sectionCode, parts) {
		code := model.SectionCode(section.Code)
		units := sectionUnits(section)
		for _, unit := range units {
			if unit.group == nil {
				for _, question := range unit.questions {
					add(code, questionPart(question), nil, []uint{question.ID})
				}
				continue
			}

			group := groups[unit.group.ID]
			part := unit.part
			if part != nil && onePagePerQuestion(*part) {
				for _, question := range unit.questions {
					add(code, part, group, []uint{question.ID})
				}
				continue
			}
			questionIDs := make([]uint, 0, len(unit.questions))
			for _, question := range unit.questions {
				questionIDs = append(questionIDs, question.ID)
			}
			add(code, part, group, questionIDs)
		}
	}

	return pages
}

// layoutUnit là một group, hoặc một câu không thuộc group, trước khi chia trang.
type layoutUnit struct {
	part      *model.PartCode
	order     int
	group     *dto.ExamQuestionGroup
	questions []*dto.ExamQuestion
}

// sectionUnits trộn group và câu lẻ của một section theo (part, order). Group trong
// section đã được dto sắp theo (part, order); câu lẻ đã sắp theo order.
func sectionUnits(section *dto.ExamQuestionsSection) []layoutUnit {
	units := make([]layoutUnit, 0, len(section.Groups)+len(section.Questions))
	for _, group := range section.Groups {
		units = append(units, layoutUnit{part: questionPart(group.Questions[0]), order: group.Order, group: group, questions: group.Questions})
	}
	for _, question := range section.Questions {
		units = append(units, layoutUnit{part: questionPart(question), order: question.Order, questions: []*dto.ExamQuestion{question}})
	}
	if len(section.Questions) == 0 || len(section.Groups) == 0 {
		return units
	}

	partKey := func(part *model.PartCode) string {
		if part == nil {
			return ""
		}
		return string(*part)
	}
	sort.SliceStable(units, func(i, j int) bool {
		a, b := units[i], units[j]
		if partKey(a.part) != partKey(b.part) {
			return partKey(a.part) < partKey(b.part)
		}
		return a.order < b.order
	})
	return units
}

func onePagePerQuestion(part model.PartCode) bool {
	return part == model.Part1 || part == model.Part2 || part == model.Part5
}

func questionPart(question *dto.ExamQuestion) *model.PartCode {
	if question.Part == nil {
		return nil
	}
	part := model.PartCode(*question.Part)
	return &part
}

// listeningTiming: audio đã có khoảng lặng giữa các câu, chỉ bù khoảng lặng cuối file
// cho đủ mục tiêu của part: gapMs = max(0, target − TrailingSilenceMs) (PLAN mục 5).
func listeningTiming(part *model.PartCode, group *model.Group, gap listeningGapConfig) (audioMs, gapMs int) {
	target := 0
	if part != nil {
		switch *part {
		case model.Part1:
			target = gap[0]
		case model.Part2:
			target = gap[1]
		case model.Part3:
			target = gap[2]
		case model.Part4:
			target = gap[3]
		}
	}
	if group == nil {
		return 0, target
	}
	if group.AudioDurationMs != nil {
		audioMs = *group.AudioDurationMs
	}
	gapMs = target
	if group.TrailingSilenceMs != nil {
		gapMs = max(0, target-*group.TrailingSilenceMs)
	}
	return audioMs, gapMs
}

// hasListeningPage cho biết attempt strict bắt đầu ở LISTENING hay READING.
func hasListeningPage(layout []model.AttemptPage) bool {
	for _, page := range layout {
		if page.Section == model.SectionCodeListening {
			return true
		}
	}
	return false
}
