package llm_test

import (
	"context"
	"os"
	"testing"
	"time"

	"ego/services/ai/internal/llm"
)

func TestGeminiChat(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY is not set")
	}
	client := llm.NewClient(apiKey, "gemini-flash-lite-latest")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	resp, err := client.Chat(ctx, "Xin chao")
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	if resp == "" {
		t.Fatal("Expected non-empty response")
	}
	t.Logf("Response: %s", resp)
}
