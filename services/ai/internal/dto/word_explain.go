package dto

// WordMeaning chứa thông tin một nghĩa của từ (phần của speech + định nghĩa + ví dụ)
type WordMeaning struct {
	PartOfSpeech string   `json:"partOfSpeech"`
	Definitions  []string `json:"definitions"` // top 2-3 definitions
	Examples     []string `json:"examples"`    // ví dụ trong câu
	Synonyms     []string `json:"synonyms"`
}

// WordExplainRequest — Flutter gửi kèm dữ liệu từ điển đã có sẵn trên client
// để AI service xây dựng RAG context mà không cần gọi lại knowledge service
type WordExplainRequest struct {
	Word     string        `json:"word"`     // từ cần giải thích (bắt buộc)
	Phonetic string        `json:"phonetic"` // phiên âm (tùy chọn)
	Meanings []WordMeaning `json:"meanings"` // các nghĩa của từ (tùy chọn nhưng nên có)
}

// WordExplainResponse — kết quả giải thích từ AI
type WordExplainResponse struct {
	Answer string `json:"answer"`
}
