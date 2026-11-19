package service

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"ego/services/ai/internal/dto"
)

type grammarLLMStub struct {
	system string
	prompt string
	err    error
}

func (f *grammarLLMStub) Chat(context.Context, string) (string, error) {
	return "", nil
}

func (f *grammarLLMStub) ChatWithSystem(_ context.Context, system, prompt string) (string, error) {
	f.system, f.prompt = system, prompt
	return "Because B uses the wrong pronoun; A agrees with am.", f.err
}

func grammarRequest() dto.GrammarExplainRequest {
	selected := 1
	return dto.GrammarExplainRequest{
		UnitTitle:           "Pronouns",
		Question:            "___ am ready.",
		Options:             []string{"I", "He"},
		Correct:             "I",
		Explanation:         "Use I with am.",
		SelectedOptionIndex: &selected,
	}
}

func TestExplainGrammarUsesSelectedAnswerAndExistingContext(t *testing.T) {
	llm := &grammarLLMStub{}
	svc := &service{llmClient: llm}
	resp, httpErr := svc.ExplainGrammar(context.Background(), grammarRequest())
	if httpErr != nil || resp == nil || !strings.Contains(resp.Answer, "Because B") {
		t.Fatalf("unexpected response: %#v, %v", resp, httpErr)
	}
	for _, part := range []string{"Question: \"___ am ready.\"", "Learner selected: B. \"He\"", "Correct answer: A. \"I\"", "Existing explanation: \"Use I with am.\""} {
		if !strings.Contains(llm.prompt, part) {
			t.Errorf("missing context %q", part)
		}
	}
	if !strings.Contains(llm.system, "Answer in English only") {
		t.Fatal("English response instruction missing")
	}
}

func TestExplainGrammarRejectsInvalidAndCorrectSelection(t *testing.T) {
	for _, mutate := range []func(*dto.GrammarExplainRequest){
		func(r *dto.GrammarExplainRequest) { r.SelectedOptionIndex = nil },
		func(r *dto.GrammarExplainRequest) { *r.SelectedOptionIndex = 2 },
		func(r *dto.GrammarExplainRequest) { r.Correct = "unknown" },
		func(r *dto.GrammarExplainRequest) { r.Explanation = "" },
		func(r *dto.GrammarExplainRequest) { *r.SelectedOptionIndex = 0 },
	} {
		req := grammarRequest()
		mutate(&req)
		llm := &grammarLLMStub{}
		_, httpErr := (&service{llmClient: llm}).ExplainGrammar(context.Background(), req)
		if httpErr == nil || httpErr.StatusCode != http.StatusBadRequest || llm.prompt != "" {
			t.Fatalf("expected 400 without calling LLM, got %v", httpErr)
		}
	}
}

func TestExplainGrammarSupportsLetterCorrectAnswerAndLLMError(t *testing.T) {
	req := grammarRequest()
	req.Correct = "A"
	llm := &grammarLLMStub{err: errors.New("provider unavailable")}
	_, httpErr := (&service{llmClient: llm}).ExplainGrammar(context.Background(), req)
	if httpErr == nil || httpErr.StatusCode != http.StatusInternalServerError || llm.prompt == "" {
		t.Fatalf("expected provider error after valid letter answer, got %v", httpErr)
	}
}
