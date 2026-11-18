package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const geminiBaseURL = "https://generativelanguage.googleapis.com/v1beta/models"

// systemPrompt là persona của AI tutor
const systemPrompt = `Bạn là một gia sư tiếng Anh thông minh, thân thiện và chuyên nghiệp.
Chuyên môn của bạn là giúp người Việt Nam học tiếng Anh, đặc biệt là từ vựng, ngữ pháp, và luyện thi TOEIC.

Nguyên tắc trả lời:
- Trả lời bằng tiếng Việt trừ khi người dùng hỏi bằng tiếng Anh hoặc yêu cầu trả lời tiếng Anh.
- Giải thích rõ ràng, dễ hiểu với ví dụ cụ thể.
- Với từ vựng: cung cấp phiên âm, nghĩa, loại từ, ví dụ trong câu.
- Với ngữ pháp: giải thích cấu trúc, công thức, cách dùng, ví dụ đúng và sai.
- Với TOEIC: hướng dẫn chiến lược làm bài, giải thích đáp án.
- Luôn khuyến khích và động viên người học.
- Trả lời ngắn gọn, súc tích nhưng đầy đủ thông tin cần thiết.`

type Client struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

func NewClient(apiKey, model string) *Client {
	if model == "" || model == "gemini-2.5-flash" || model == "gemini-2.5-flash-lite" {
		model = "gemini-flash-lite-latest"
	}
	return &Client{
		apiKey: apiKey,
		model:  model,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// geminiRequest là cấu trúc gửi lên Gemini REST API
type geminiRequest struct {
	SystemInstruction *systemInstruction `json:"system_instruction,omitempty"`
	Contents          []content          `json:"contents"`
	GenerationConfig  generationConfig   `json:"generationConfig"`
}

type systemInstruction struct {
	Parts []part `json:"parts"`
}

type content struct {
	Parts []part `json:"parts"`
	Role  string `json:"role,omitempty"`
}

type part struct {
	Text string `json:"text"`
}

type generationConfig struct {
	Temperature     float64 `json:"temperature"`
	MaxOutputTokens int     `json:"maxOutputTokens"`
}

// geminiResponse là cấu trúc response từ Gemini REST API
type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"error"`
}

// ChatWithSystem gửi message tới Gemini với system prompt tuỳ chỉnh (dùng cho RAG)
func (c *Client) ChatWithSystem(ctx context.Context, customSystemPrompt, userMessage string) (string, error) {
	candidates := []string{
		"gemini-flash-lite-latest",
		"gemini-3.5-flash-lite",
		c.model,
	}
	seen := make(map[string]bool)
	var modelsToTry []string
	for _, m := range candidates {
		if m != "" && m != "gemini-2.5-flash" && m != "gemini-2.5-flash-lite" && !seen[m] {
			seen[m] = true
			modelsToTry = append(modelsToTry, m)
		}
	}

	var lastErr error
	for _, model := range modelsToTry {
		subCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
		text, err := c.generateWithSystemAndModel(subCtx, model, customSystemPrompt, userMessage)
		cancel()
		if err == nil {
			return text, nil
		}
		lastErr = err
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
	return "", lastErr
}

func (c *Client) generateWithSystemAndModel(ctx context.Context, model, sysPrompt, userMessage string) (string, error) {
	reqBody := geminiRequest{
		SystemInstruction: &systemInstruction{
			Parts: []part{{Text: sysPrompt}},
		},
		Contents: []content{
			{
				Role:  "user",
				Parts: []part{{Text: userMessage}},
			},
		},
		GenerationConfig: generationConfig{
			Temperature:     0.7,
			MaxOutputTokens: 1024,
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/%s:generateContent?key=%s", geminiBaseURL, model, c.apiKey)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to call Gemini API: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var geminiResp geminiResponse
	if err := json.Unmarshal(respBytes, &geminiResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if geminiResp.Error != nil {
		return "", fmt.Errorf("gemini API error %d: %s", geminiResp.Error.Code, geminiResp.Error.Message)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty response from Gemini")
	}

	return geminiResp.Candidates[0].Content.Parts[0].Text, nil
}

func (c *Client) Chat(ctx context.Context, userMessage string) (string, error) {
	// Ưu tiên các model phản hồi nhanh và khả dụng cao trên Google Gemini API hiện tại
	candidates := []string{
		"gemini-flash-lite-latest",
		"gemini-3.5-flash-lite",
		c.model,
	}
	seen := make(map[string]bool)
	var modelsToTry []string
	for _, m := range candidates {
		// Loại bỏ các model đã bị Google ngừng hỗ trợ (404 Not Found)
		if m != "" && m != "gemini-2.5-flash" && m != "gemini-2.5-flash-lite" && !seen[m] {
			seen[m] = true
			modelsToTry = append(modelsToTry, m)
		}
	}

	var lastErr error
	for _, model := range modelsToTry {
		subCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
		text, err := c.generateWithModel(subCtx, model, userMessage)
		cancel()
		if err == nil {
			return text, nil
		}
		lastErr = err
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
	return "", lastErr
}

func (c *Client) generateWithModel(ctx context.Context, model, userMessage string) (string, error) {
	reqBody := geminiRequest{
		SystemInstruction: &systemInstruction{
			Parts: []part{{Text: systemPrompt}},
		},
		Contents: []content{
			{
				Role:  "user",
				Parts: []part{{Text: userMessage}},
			},
		},
		GenerationConfig: generationConfig{
			Temperature:     0.7,
			MaxOutputTokens: 1024,
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/%s:generateContent?key=%s", geminiBaseURL, model, c.apiKey)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to call Gemini API: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var geminiResp geminiResponse
	if err := json.Unmarshal(respBytes, &geminiResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if geminiResp.Error != nil {
		return "", fmt.Errorf("gemini API error %d: %s", geminiResp.Error.Code, geminiResp.Error.Message)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty response from Gemini")
	}

	return geminiResp.Candidates[0].Content.Parts[0].Text, nil
}
