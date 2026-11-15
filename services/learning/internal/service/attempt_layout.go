package service

import (
	"sort"

	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/model"
)

// listeningGapConfig là thời gian trả lời mục tiêu sau audio của trang nghe, theo part 1..4 (ms).
type listeningGapConfig [4]int

// defaultListeningGap là thời gian trả lời mặc định sau audio cho part 1–4 (ms): part 1, 2 câu
// ngắn nên ít hơn part 3, 4 (nhiều câu trên một đoạn).
var defaultListeningGap = listeningGapConfig{5000, 5000, 8000, 8000}

// listeningGap lấy thời gian trả lời sau audio theo part từ config, để chỉnh được theo môi
// trường mà không sửa code; service không có config (test) thì dùng giá trị mặc định.
func (s *service) listeningGap() listeningGapConfig {
	if s.cfg == nil {
		return defaultListeningGap
	}
	return s.cfg.TOEICListeningGapMs
}

// attemptFlowFor quyết định lượt làm bài mới chạy theo luồng nào. Luồng quy định học viên
// được di chuyển giữa các trang ra sao:
//   - FREE: mở trang nào cũng được, quay lại sửa đáp án lúc nào cũng được, chỉ bị giới hạn
//     bởi tổng thời gian làm bài.
//   - TOEIC_STRICT: mô phỏng phòng thi TOEIC. Phần nghe chạy trước, mỗi trang nghe tự chuyển
//     sang trang sau khi hết audio và thời gian trả lời; trang đã qua bị khóa, không quay lại
//     được. Nghe xong mới được làm phần đọc.
//
// Chỉ đề TOEIC ở chế độ TEST hoặc PRACTICE mới dùng TOEIC_STRICT, vì chỉ TOEIC có phần nghe
// cần chạy theo nhịp audio. Bài tập lớp (chế độ ASSIGNMENT) và đề THPT luôn là FREE.
//
// Luồng mới còn được bật/tắt bằng cờ TOEIC_STRICT_FLOW_ENABLED: cờ tắt thì mọi lượt là FREE.
// Service không có config (trong test) cũng coi như cờ tắt.
//
// Kết quả được lưu vào attempt lúc tạo và không đổi về sau, nên bật/tắt cờ chỉ ảnh hưởng các
// lượt tạo sau đó, không làm đổi luật của lượt đang làm.
func (s *service) attemptFlowFor(exam *model.Exam, mode model.AttemptMode) model.AttemptFlow {
	if s.cfg != nil && s.cfg.TOEICStrictFlowEnabled &&
		exam.Type == model.ExamTypeTOEIC &&
		(mode == model.AttemptModeTest || mode == model.AttemptModePractice) {
		return model.AttemptFlowTOEICStrict
	}
	return model.AttemptFlowFree
}

// buildAttemptLayout chia các câu trong phạm vi của lượt làm bài (sectionCode, parts) thành
// danh sách trang. Layout được dựng một lần lúc tạo attempt và lưu lại, để bảng chọn câu, khóa
// trang, tiến trang nghe và xem lại đều dựa trên cùng một cách chia, kể cả khi đề bị sửa sau đó.
func buildAttemptLayout(exam *model.Exam, sectionCode *model.SectionCode, parts []model.PartCode, gap listeningGapConfig) []model.AttemptPage {
	// Tra group theo ID từ model (có thông tin audio), vì DTO dùng để duyệt không mang các field đó.
	groups := make(map[uint]*model.Group)
	for i := range exam.Sections {
		for j := range exam.Sections[i].Groups {
			group := &exam.Sections[i].Groups[j]
			groups[group.ID] = group
		}
	}

	// add thêm một trang, đánh Index theo vị trí và gắn thời gian nghe nếu là trang nghe.
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
		// Trang nghe gắn độ dài audio và thời gian trả lời sau audio, là căn cứ để server tự
		// chuyển trang ở luồng TOEIC_STRICT.
		if section == model.SectionCodeListening {
			page.AudioMs, page.GapMs = listeningTiming(part, group, gap)
		}
		pages = append(pages, page)
	}

	// Duyệt theo đúng thứ tự hiển thị của đề (section, part, group, câu) đã lọc theo phạm vi.
	// Section nghe đứng trước nên các trang nghe luôn là đoạn đầu của layout.
	for _, section := range dto.ToAttemptSections(exam, sectionCode, parts) {
		code := model.SectionCode(section.Code)
		units := sectionUnits(section)
		for _, unit := range units {
			// Câu không thuộc group: mỗi câu một trang.
			if unit.group == nil {
				for _, question := range unit.questions {
					add(code, questionPart(question), nil, []uint{question.ID})
				}
				continue
			}

			// Group của part 1, 2, 5: câu đơn, mỗi câu tự đứng được nên mỗi câu một trang.
			group := groups[unit.group.ID]
			part := unit.part
			if part != nil && onePagePerQuestion(*part) {
				for _, question := range unit.questions {
					add(code, part, group, []uint{question.ID})
				}
				continue
			}
			// Group của part khác: nhiều câu chung một đoạn nghe/đọc nên gom cả group vào một
			// trang, để học viên thấy đoạn văn hoặc nghe audio cùng lúc với mọi câu của nó.
			questionIDs := make([]uint, 0, len(unit.questions))
			for _, question := range unit.questions {
				questionIDs = append(questionIDs, question.ID)
			}
			add(code, part, group, questionIDs)
		}
	}

	return pages
}

// layoutUnit là một đơn vị trung gian khi chia trang, chỉ dùng trong lúc dựng layout: một group
// kèm mọi câu của nó, hoặc một câu đứng riêng không thuộc group. sectionUnits sắp các đơn vị
// theo thứ tự trên đề, rồi buildAttemptLayout quyết định mỗi đơn vị thành một hay nhiều trang.
//   - part: part của đơn vị (của group thì lấy theo câu đầu tiên); khóa sắp xếp thứ nhất, và
//     quyết định group có bị tách mỗi câu một trang không (part 1, 2, 5).
//   - order: thứ tự của group hoặc của câu trong section; khóa sắp xếp thứ hai.
//   - group: group của đơn vị; nil với câu đứng riêng.
//   - questions: các câu của đơn vị, theo thứ tự hiển thị; câu đứng riêng thì chỉ có một câu.
type layoutUnit struct {
	part      *model.PartCode
	order     int
	group     *dto.ExamQuestionGroup
	questions []*dto.ExamQuestion
}

// sectionUnits gom group và câu đứng riêng của một section thành một danh sách đơn vị, sắp
// theo đúng thứ tự trên đề, để buildAttemptLayout chia trang theo thứ tự đó.
// DTO tách group và câu đứng riêng thành hai danh sách đã sắp sẵn, nhưng trên đề hai loại
// xen kẽ nhau (vd trong section đọc, câu đứng riêng của part 5 phải đứng trước các group của
// part 6, 7), nên cần trộn lại.
func sectionUnits(section *dto.ExamQuestionsSection) []layoutUnit {
	// Part của group lấy theo câu đầu tiên, vì mọi câu trong một group cùng part.
	units := make([]layoutUnit, 0, len(section.Groups)+len(section.Questions))
	for _, group := range section.Groups {
		units = append(units, layoutUnit{part: questionPart(group.Questions[0]), order: group.Order, group: group, questions: group.Questions})
	}
	for _, question := range section.Questions {
		units = append(units, layoutUnit{part: questionPart(question), order: question.Order, questions: []*dto.ExamQuestion{question}})
	}
	// Chỉ có một loại thì danh sách đã đúng thứ tự, không cần sắp lại.
	if len(section.Questions) == 0 || len(section.Groups) == 0 {
		return units
	}

	// Sắp theo part trước rồi tới order trong part. Câu không có part xếp đầu. Sắp ổn định để
	// hai đơn vị trùng khóa giữ thứ tự gốc. Part là chuỗi một chữ số nên so chuỗi là đúng.

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

// onePagePerQuestion cho biết group thuộc part này có bị tách thành mỗi câu một trang không.
// Part 1 (mô tả tranh), part 2 (hỏi đáp) và part 5 (điền câu) là câu đơn: mỗi câu tự đứng được,
// không dùng chung đoạn nghe hay đoạn đọc với câu khác, nên mỗi câu một trang.
// Part 3, 4, 6, 7 có nhiều câu hỏi trên cùng một đoạn hội thoại, bài nói hoặc bài đọc, nên giữ
// cả group trên một trang để học viên thấy đoạn đó cùng lúc với mọi câu của nó.
func onePagePerQuestion(part model.PartCode) bool {
	return part == model.Part1 || part == model.Part2 || part == model.Part5
}

// questionPart lấy part của câu từ DTO và đổi sang model.PartCode, vì DTO lưu part dạng chuỗi
// còn layout và các hàm chia trang làm việc với PartCode. Trả nil khi câu không có part (đề
// không chia part), để caller phân biệt được "không có part" với một part cụ thể.
func questionPart(question *dto.ExamQuestion) *model.PartCode {
	if question.Part == nil {
		return nil
	}
	part := model.PartCode(*question.Part)
	return &part
}

// listeningTiming tính thời gian của một trang nghe: audioMs là độ dài file audio, gapMs là
// thời gian chờ thêm sau khi audio hết để học viên trả lời. Server tự chuyển trang khi đã qua
// audioMs + gapMs (cộng thời gian dư).
func listeningTiming(part *model.PartCode, group *model.Group, gap listeningGapConfig) (audioMs, gapMs int) {
	// Thời gian trả lời mục tiêu lấy theo part 1–4 từ config. Part khác (hoặc không có part)
	// thì không có thời gian chờ.
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
	// Không có group thì không có audio, chỉ còn thời gian trả lời.
	if group == nil {
		return 0, target
	}
	if group.AudioDurationMs != nil {
		audioMs = *group.AudioDurationMs
	}
	// File audio thường đã có sẵn một đoạn lặng ở cuối. Đoạn đó đã nằm trong audioMs nên chỉ
	// bù phần còn thiếu cho đủ mục tiêu, không cộng thêm nguyên target: gapMs = max(0,
	// target − TrailingSilenceMs). Chưa đo được khoảng lặng thì chờ đủ target.
	gapMs = target
	if group.TrailingSilenceMs != nil {
		gapMs = max(0, target-*group.TrailingSilenceMs)
	}
	return audioMs, gapMs
}

// hasListeningPage cho biết layout có trang nghe nào không. Dùng lúc tạo lượt TOEIC_STRICT để
// chọn stage bắt đầu: có trang nghe thì bắt đầu ở LISTENING, không có (vd chỉ làm phần đọc
// hoặc chỉ chọn part 5–7) thì vào thẳng READING, tránh kẹt ở phần nghe rỗng.
func hasListeningPage(layout []model.AttemptPage) bool {
	for _, page := range layout {
		if page.Section == model.SectionCodeListening {
			return true
		}
	}
	return false
}
