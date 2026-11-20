package model

// Unique theo từ (đã lưu chữ thường) với các dòng chưa xóa mềm. text_pattern_ops
// để index phục vụ được cả `word = ?` lẫn tìm theo tiền tố `word LIKE 'he%'`.
const CreateWordUniqueIndexSQL = `
	CREATE UNIQUE INDEX IF NOT EXISTS idx_words_word_unique
	ON words (word text_pattern_ops)
	WHERE deleted_at IS NULL;
`

// Composite unique: một từ chỉ xuất hiện 1 lần trong 1 deck (soft-delete safe).
const CreateFlashcardDeckWordUniqueIndexSQL = `
	CREATE UNIQUE INDEX IF NOT EXISTS idx_flashcards_deck_word_unique
	ON flashcards (deck_id, word_id)
	WHERE deleted_at IS NULL;
`
