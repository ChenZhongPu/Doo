package adapter

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

type mockClock struct {
	mu  sync.Mutex
	now time.Time
}

func newMockClock() *mockClock {
	return &mockClock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
}

func (m *mockClock) Now() time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.now
}

func (m *mockClock) Advance(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.now = m.now.Add(d)
}

func TestOpenAIChatAdapter_Success_WithReasoningAndUsage(t *testing.T) {
	clock := newMockClock()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}
		if r.Header.Get("X-Custom") != "custom-val" {
			t.Errorf("missing custom header")
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)

		clock.Advance(100 * time.Millisecond)
		fmt.Fprintf(w, "data: {\"id\":\"chatcmpl-1\",\"model\":\"gpt-4o\",\"choices\":[{\"delta\":{\"reasoning\":\"thinking part 1 \"}}]}\n\n")
		flusher.Flush()

		time.Sleep(10 * time.Millisecond)
		clock.Advance(100 * time.Millisecond)
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Hello \"}}]}\n\n")
		flusher.Flush()

		time.Sleep(10 * time.Millisecond)
		clock.Advance(50 * time.Millisecond)
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"thinking part 2\"}}]}\n\n")
		flusher.Flush()

		time.Sleep(10 * time.Millisecond)
		clock.Advance(50 * time.Millisecond)
		finish := "stop"
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\"world!\"},\"finish_reason\":\"%s\"}]}\n\n", finish)
		flusher.Flush()

		time.Sleep(10 * time.Millisecond)
		clock.Advance(50 * time.Millisecond)
		fmt.Fprintf(w, "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":25,\"completion_tokens_details\":{\"reasoning_tokens\":12}}}\n\n")
		flusher.Flush()

		// 6. [DONE]
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer ts.Close()

	adapter := &OpenAIChatAdapter{
		Clock:  clock.Now,
		Client: ts.Client(),
	}

	p := Provider{
		BaseURL:  ts.URL,
		Protocol: "openai_chat",
		Headers:  map[string]string{"X-Custom": "custom-val"},
	}

	var events []Event
	res := adapter.Do(context.Background(), p, "test-key", []byte(`{}`), func(e Event) {
		events = append(events, e)
	})

	if res.Status != "done" {
		t.Fatalf("expected status done, got %s, err: %s", res.Status, res.Err)
	}
	if res.HTTPStatus != 200 {
		t.Errorf("expected http status 200, got %d", res.HTTPStatus)
	}
	if res.ResponseModel != "gpt-4o" {
		t.Errorf("expected model gpt-4o, got %s", res.ResponseModel)
	}
	if res.ProviderReqID != "chatcmpl-1" {
		t.Errorf("expected id chatcmpl-1, got %s", res.ProviderReqID)
	}
	if res.Output != "Hello world!" {
		t.Errorf("expected output 'Hello world!', got %q", res.Output)
	}
	if res.Reasoning != "thinking part 1 thinking part 2" {
		t.Errorf("expected reasoning 'thinking part 1 thinking part 2', got %q", res.Reasoning)
	}
	if res.FinishReason != "stop" {
		t.Errorf("expected finish_reason 'stop', got %q", res.FinishReason)
	}
	if res.PromptTokens == nil || *res.PromptTokens != 10 {
		t.Errorf("expected 10 prompt tokens, got %v", res.PromptTokens)
	}
	if res.CompletionTokens == nil || *res.CompletionTokens != 25 {
		t.Errorf("expected 25 completion tokens, got %v", res.CompletionTokens)
	}
	if res.ReasoningTokens == nil || *res.ReasoningTokens != 12 {
		t.Errorf("expected 12 reasoning tokens, got %v", res.ReasoningTokens)
	}

	// Verify timing: reasoning arrived first, so first_token < first_content
	if res.Metrics.FirstTokenMs == nil || res.Metrics.FirstContentMs == nil {
		t.Fatalf("timing metrics missing: %+v", res.Metrics)
	}
	if *res.Metrics.FirstTokenMs >= *res.Metrics.FirstContentMs {
		t.Errorf("expected first_token (%v) < first_content (%v)", *res.Metrics.FirstTokenMs, *res.Metrics.FirstContentMs)
	}
}

func TestOpenAIChatAdapter_Error429(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprintf(w, `{"error":{"message":"Rate limit reached","type":"rate_limit_error"}}`)
	}))
	defer ts.Close()

	adapter := &OpenAIChatAdapter{Client: ts.Client()}
	p := Provider{BaseURL: ts.URL, Protocol: "openai_chat"}

	res := adapter.Do(context.Background(), p, "key", []byte(`{}`), nil)
	if res.Status != "error" {
		t.Fatalf("expected status error, got %s", res.Status)
	}
	if res.HTTPStatus != 429 {
		t.Errorf("expected http 429, got %d", res.HTTPStatus)
	}
	if res.Err != "Rate limit reached" {
		t.Errorf("expected error message 'Rate limit reached', got %q", res.Err)
	}
}

func TestOpenAIChatAdapter_ChunkError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\"start\"}}]}\n\n")
		fmt.Fprintf(w, "data: {\"error\":\"something failed upstream\"}\n\n")
	}))
	defer ts.Close()

	adapter := &OpenAIChatAdapter{Client: ts.Client()}
	p := Provider{BaseURL: ts.URL, Protocol: "openai_chat"}

	res := adapter.Do(context.Background(), p, "key", []byte(`{}`), nil)
	if res.Status != "error" {
		t.Fatalf("expected status error, got %s", res.Status)
	}
	if res.Output != "start" {
		t.Errorf("expected accumulated output 'start', got %q", res.Output)
	}
}

func TestOpenAIChatAdapter_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\"partial output\"}}]}\n\n")
		flusher.Flush()
		// Wait for cancel
		<-r.Context().Done()
	}))
	defer ts.Close()

	adapter := &OpenAIChatAdapter{Client: ts.Client()}
	p := Provider{BaseURL: ts.URL, Protocol: "openai_chat"}

	done := make(chan *Result)
	go func() {
		res := adapter.Do(ctx, p, "key", []byte(`{}`), func(e Event) {
			if e.Text == "partial output" {
				cancel()
			}
		})
		done <- res
	}()

	res := <-done
	if res.Status != "canceled" {
		t.Fatalf("expected status canceled, got %s", res.Status)
	}
	if res.Output != "partial output" {
		t.Errorf("expected partial output retained, got %q", res.Output)
	}
}

func TestAnthropicAdapter_Success_WithThinkingAndUsage(t *testing.T) {
	clock := newMockClock()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/messages" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "claude-key" {
			t.Errorf("unexpected x-api-key: %s", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("anthropic-version") != "2023-06-01" {
			t.Errorf("unexpected anthropic-version: %s", r.Header.Get("anthropic-version"))
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)

		// 1. message_start
		clock.Advance(100 * time.Millisecond)
		fmt.Fprintf(w, "data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_123\",\"model\":\"claude-3-7-sonnet\",\"usage\":{\"input_tokens\":40}}}\n\n")
		flusher.Flush()

		// 2. thinking delta
		time.Sleep(10 * time.Millisecond)
		clock.Advance(50 * time.Millisecond)
		fmt.Fprintf(w, "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"thinking_delta\",\"thinking\":\"Let me think about this\"}}\n\n")
		flusher.Flush()

		// 3. text delta
		time.Sleep(10 * time.Millisecond)
		clock.Advance(50 * time.Millisecond)
		fmt.Fprintf(w, "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"Here is the response\"}}\n\n")
		flusher.Flush()

		// 4. message_delta
		time.Sleep(10 * time.Millisecond)
		clock.Advance(50 * time.Millisecond)
		fmt.Fprintf(w, "data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":65}}\n\n")
		flusher.Flush()

		// 5. message_stop
		fmt.Fprintf(w, "data: {\"type\":\"message_stop\"}\n\n")
		flusher.Flush()
	}))
	defer ts.Close()

	adapter := &AnthropicAdapter{
		Clock:  clock.Now,
		Client: ts.Client(),
	}

	p := Provider{
		BaseURL:  ts.URL,
		Protocol: "anthropic",
	}

	res := adapter.Do(context.Background(), p, "claude-key", []byte(`{}`), nil)

	if res.Status != "done" {
		t.Fatalf("expected status done, got %s, err: %s", res.Status, res.Err)
	}
	if res.ResponseModel != "claude-3-7-sonnet" {
		t.Errorf("expected claude-3-7-sonnet, got %s", res.ResponseModel)
	}
	if res.ProviderReqID != "msg_123" {
		t.Errorf("expected msg_123, got %s", res.ProviderReqID)
	}
	if res.Output != "Here is the response" {
		t.Errorf("expected output, got %q", res.Output)
	}
	if res.Reasoning != "Let me think about this" {
		t.Errorf("expected reasoning, got %q", res.Reasoning)
	}
	if res.FinishReason != "end_turn" {
		t.Errorf("expected finish_reason end_turn, got %q", res.FinishReason)
	}
	if res.PromptTokens == nil || *res.PromptTokens != 40 {
		t.Errorf("expected 40 prompt tokens, got %v", res.PromptTokens)
	}
	if res.CompletionTokens == nil || *res.CompletionTokens != 65 {
		t.Errorf("expected 65 completion tokens, got %v", res.CompletionTokens)
	}
	if res.ReasoningTokens != nil {
		t.Errorf("expected reasoning_tokens to be nil for Anthropic, got %v", res.ReasoningTokens)
	}
	if res.Metrics.FirstTokenMs == nil || res.Metrics.FirstContentMs == nil {
		t.Fatalf("metrics missing: %+v", res.Metrics)
	}
	if *res.Metrics.FirstTokenMs >= *res.Metrics.FirstContentMs {
		t.Errorf("expected first_token < first_content")
	}
}

func TestAnthropicAdapter_ErrorEvent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "data: {\"type\":\"error\",\"error\":{\"message\":\"overloaded\"}}\n\n")
	}))
	defer ts.Close()

	adapter := &AnthropicAdapter{Client: ts.Client()}
	p := Provider{BaseURL: ts.URL, Protocol: "anthropic"}

	res := adapter.Do(context.Background(), p, "key", []byte(`{}`), nil)
	if res.Status != "error" {
		t.Fatalf("expected status error, got %s", res.Status)
	}
	if res.Err != "overloaded" {
		t.Errorf("expected overloaded error message, got %q", res.Err)
	}
}

func TestOpenAIChatAdapter_NonStreaming_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{
			"id": "chatcmpl-nonstream-1",
			"model": "deepseek-chat",
			"choices": [{
				"message": {
					"role": "assistant",
					"content": "Non-stream response",
					"reasoning_content": "DeepSeek thought"
				},
				"finish_reason": "stop"
			}],
			"usage": {
				"prompt_tokens": 15,
				"completion_tokens": 20,
				"completion_tokens_details": {
					"reasoning_tokens": 8
				}
			}
		}`)
	}))
	defer ts.Close()

	adapter := &OpenAIChatAdapter{Client: ts.Client()}
	p := Provider{BaseURL: ts.URL, Protocol: "openai_chat"}

	var emittedTokens, emittedReasoning []string
	res := adapter.Do(context.Background(), p, "key", []byte(`{}`), func(e Event) {
		if e.Kind == "content" {
			emittedTokens = append(emittedTokens, e.Text)
		} else if e.Kind == "reasoning" {
			emittedReasoning = append(emittedReasoning, e.Text)
		}
	})

	if res.Status != "done" {
		t.Fatalf("expected status done, got %s (err: %s)", res.Status, res.Err)
	}
	if res.Output != "Non-stream response" {
		t.Errorf("expected Output 'Non-stream response', got %q", res.Output)
	}
	if res.Reasoning != "DeepSeek thought" {
		t.Errorf("expected Reasoning 'DeepSeek thought', got %q", res.Reasoning)
	}
	if res.ResponseModel != "deepseek-chat" {
		t.Errorf("expected ResponseModel 'deepseek-chat', got %q", res.ResponseModel)
	}
	if res.ProviderReqID != "chatcmpl-nonstream-1" {
		t.Errorf("expected ProviderReqID 'chatcmpl-nonstream-1', got %q", res.ProviderReqID)
	}
	if res.FinishReason != "stop" {
		t.Errorf("expected FinishReason 'stop', got %q", res.FinishReason)
	}
	if res.PromptTokens == nil || *res.PromptTokens != 15 {
		t.Errorf("expected PromptTokens 15, got %v", res.PromptTokens)
	}
	if res.CompletionTokens == nil || *res.CompletionTokens != 20 {
		t.Errorf("expected CompletionTokens 20, got %v", res.CompletionTokens)
	}
	if res.ReasoningTokens == nil || *res.ReasoningTokens != 8 {
		t.Errorf("expected ReasoningTokens 8, got %v", res.ReasoningTokens)
	}
}

func TestAnthropicAdapter_NonStreaming_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{
			"id": "msg_nonstream-1",
			"model": "claude-3-5-sonnet-20241022",
			"type": "message",
			"role": "assistant",
			"content": [
				{
					"type": "thinking",
					"thinking": "Claude thoughts"
				},
				{
					"type": "text",
					"text": "Hello from non-stream Claude!"
				}
			],
			"stop_reason": "end_turn",
			"usage": {
				"input_tokens": 12,
				"output_tokens": 18
			}
		}`)
	}))
	defer ts.Close()

	adapter := &AnthropicAdapter{Client: ts.Client()}
	p := Provider{BaseURL: ts.URL, Protocol: "anthropic"}

	res := adapter.Do(context.Background(), p, "key", []byte(`{}`), nil)
	if res.Status != "done" {
		t.Fatalf("expected status done, got %s (err: %s)", res.Status, res.Err)
	}
	if res.Output != "Hello from non-stream Claude!" {
		t.Errorf("unexpected output: %q", res.Output)
	}
	if res.Reasoning != "Claude thoughts" {
		t.Errorf("unexpected reasoning: %q", res.Reasoning)
	}
	if res.FinishReason != "end_turn" {
		t.Errorf("unexpected finish reason: %q", res.FinishReason)
	}
	if res.PromptTokens == nil || *res.PromptTokens != 12 {
		t.Errorf("unexpected prompt tokens: %v", res.PromptTokens)
	}
	if res.CompletionTokens == nil || *res.CompletionTokens != 18 {
		t.Errorf("unexpected completion tokens: %v", res.CompletionTokens)
	}
}
