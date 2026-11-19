package service

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"ego/platform/httpx"
	"ego/services/ai/internal/dto"
	"ego/services/ai/internal/llm"
)

// wordTutorSystemPrompt là system prompt chuyên biệt cho RAG giải thích từ vựng
const wordTutorSystemPrompt = `Bạn là chuyên gia từ vựng tiếng Anh, hỗ trợ người Việt Nam học tiếng Anh.
Nhiệm vụ của bạn: giải thích chi tiết cách dùng từ trong văn cảnh giao tiếp thực tế.

Quy tắc trả lời:
- Viết bằng tiếng Việt, dùng tiếng Anh cho ví dụ câu.
- Giải thích rõ sắc thái nghĩa, khi nào nên/không nên dùng.
- Luôn đưa ra đúng 2 ví dụ câu hoàn chỉnh có dịch nghĩa.
- Có thể đề cập collocation (cụm từ hay đi kèm) nếu hữu ích.
- Trả lời ngắn gọn, có cấu trúc rõ ràng (dùng emoji hoặc dấu đầu dòng).`

type Service interface {
	Chat(ctx context.Context, req dto.ChatRequest) (*dto.ChatResponse, *httpx.HTTPError)
	ExplainWord(ctx context.Context, req dto.WordExplainRequest) (*dto.WordExplainResponse, *httpx.HTTPError)
	ExplainGrammar(ctx context.Context, req dto.GrammarExplainRequest) (*dto.GrammarExplainResponse, *httpx.HTTPError)
}

type service struct {
	llmClient interface {
		Chat(ctx context.Context, userMessage string) (string, error)
		ChatWithSystem(ctx context.Context, customSystemPrompt, userMessage string) (string, error)
	}
}

func New(llmClient *llm.Client) Service {
	return &service{llmClient: llmClient}
}

func (s *service) Chat(ctx context.Context, req dto.ChatRequest) (*dto.ChatResponse, *httpx.HTTPError) {
	answer, err := s.llmClient.Chat(ctx, req.Message)
	if err != nil {
		return nil, httpx.NewError(http.StatusInternalServerError, "[AI] Failed to get response from AI", err)
	}
	return &dto.ChatResponse{Answer: answer}, nil
}

// ExplainWord xây dựng RAG context từ dữ liệu từ điển Flutter gửi lên
// rồi gọi Gemini với system prompt chuyên biệt về từ vựng
func (s *service) ExplainWord(ctx context.Context, req dto.WordExplainRequest) (*dto.WordExplainResponse, *httpx.HTTPError) {
	if strings.TrimSpace(req.Word) == "" {
		return nil, httpx.NewError(http.StatusBadRequest, "word is required", nil)
	}

	// Xây dựng RAG context từ dữ liệu từ điển
	ragContext := buildWordContext(req)

	// Tạo prompt người dùng kết hợp ngữ cảnh RAG
	userPrompt := fmt.Sprintf(
		"%s\n\nGiải thích chi tiết cách dùng từ \"%s\" trong văn cảnh giao tiếp và cho 2 ví dụ câu hoàn chỉnh.",
		ragContext, req.Word,
	)

	answer, err := s.llmClient.ChatWithSystem(ctx, wordTutorSystemPrompt, userPrompt)
	if err != nil {
		return nil, httpx.NewError(http.StatusInternalServerError, "[AI] Failed to explain word", err)
	}
	return &dto.WordExplainResponse{Answer: answer}, nil
}

// buildWordContext chuyển dữ liệu từ điển thành đoạn văn bản context cho RAG
func buildWordContext(req dto.WordExplainRequest) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("=== Thông tin từ điển: \"%s\" ===\n", req.Word))
	if req.Phonetic != "" {
		sb.WriteString(fmt.Sprintf("Phiên âm: %s\n", req.Phonetic))
	}
	for i, m := range req.Meanings {
		if i >= 3 {
			break // giới hạn 3 nghĩa để tránh context quá dài
		}
		sb.WriteString(fmt.Sprintf("\n[%s]\n", m.PartOfSpeech))
		for j, def := range m.Definitions {
			if j >= 3 {
				break
			}
			sb.WriteString(fmt.Sprintf("  - Nghĩa: %s\n", def))
		}
		for j, ex := range m.Examples {
			if j >= 2 {
				break
			}
			sb.WriteString(fmt.Sprintf("  - Ví dụ: %s\n", ex))
		}
		if len(m.Synonyms) > 0 {
			syns := m.Synonyms
			if len(syns) > 4 {
				syns = syns[:4]
			}
			sb.WriteString(fmt.Sprintf("  - Từ đồng nghĩa: %s\n", strings.Join(syns, ", ")))
		}
	}
	return sb.String()
}
