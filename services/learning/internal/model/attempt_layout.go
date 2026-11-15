package model

// AttemptPage là một trang của lượt làm bài: nhóm câu hỏi học viên thấy cùng lúc trên màn hình.
// Danh sách trang được dựng một lần lúc tạo attempt (buildAttemptLayout) và lưu ở dạng JSON
// trong Attempt.Layout, nên sửa đề về sau không làm đổi cách chia trang của bài đang làm.
//   - Index: vị trí trang trong layout (từ 0), dùng trong API lấy trang và khi tiến trang nghe.
//   - Section: phần thi của trang. Ở luồng TOEIC_STRICT, Section quyết định trang có bị khóa
//     không (trang nghe chỉ mở khi đang tới lượt, trang đọc chỉ mở sau phần nghe).
//   - Part: part của trang, dùng khi trả nội dung trang (part 1, 2 ẩn nội dung câu hỏi, học
//     viên chỉ nghe). nil với câu không có part.
//   - GroupID: nhóm câu chung đoạn nghe/đọc của trang; nil với câu đứng riêng.
//   - QuestionIDs: các câu trên trang, theo thứ tự hiển thị.
//   - AudioMs, GapMs: chỉ có ở trang nghe. Độ dài audio và thời gian trả lời sau audio (ms);
//     hết audio + GapMs (cộng thời gian dư) thì server tự chuyển sang trang nghe sau.
type AttemptPage struct {
	Index       int         `json:"index"`
	Section     SectionCode `json:"section"`
	Part        *PartCode   `json:"part,omitempty"`
	GroupID     *uint       `json:"groupId,omitempty"`
	QuestionIDs []uint      `json:"questionIds"`
	AudioMs     int         `json:"audioMs"`
	GapMs       int         `json:"gapMs"`
}
