package model

// Unique theo từ (đã lưu chữ thường) với các dòng chưa xóa mềm. text_pattern_ops
// để index phục vụ được cả `word = ?` lẫn tìm theo tiền tố `word LIKE 'he%'`.
const CreateWordUniqueIndexSQL = `
	CREATE UNIQUE INDEX IF NOT EXISTS idx_words_word_unique
	ON words (word text_pattern_ops)
	WHERE deleted_at IS NULL;
`
