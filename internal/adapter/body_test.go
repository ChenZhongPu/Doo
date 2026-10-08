package adapter

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func floatPtr(f float64) *float64 { return &f }
func intPtr(i int) *int             { return &i }
func int64Ptr(i int64) *int64       { return &i }
func boolPtr(b bool) *bool          { return &b }

func rawContent(s string) json.RawMessage {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	return bytes.TrimRight(buf.Bytes(), "\n")
}

func TestBuildBody_OpenAI_UnsetFieldsDoNotAppear(t *testing.T) {
	p := Provider{Protocol: "openai_chat"}
	tgt := Target{
		ModelName: "gpt-4o",
		Core:      CoreParams{}, // all nil
	}
	in := Input{
		Messages: []Message{
			{Role: "user", Content: rawContent("hello")},
		},
	}

	body, err := BuildBody(p, tgt, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	bodyStr := string(body)
	if strings.Contains(bodyStr, "temperature") {
		t.Errorf("expected no temperature in body, got: %s", bodyStr)
	}
	if strings.Contains(bodyStr, "top_p") {
		t.Errorf("expected no top_p in body, got: %s", bodyStr)
	}
	if strings.Contains(bodyStr, "max_tokens") {
		t.Errorf("expected no max_tokens in body, got: %s", bodyStr)
	}
	if strings.Contains(bodyStr, "seed") {
		t.Errorf("expected no seed in body, got: %s", bodyStr)
	}
	if strings.Contains(bodyStr, "stream") {
		t.Errorf("expected no stream in body when unset, got: %s", bodyStr)
	}
	if strings.Contains(bodyStr, "stream_options") {
		t.Errorf("expected no stream_options when StreamUsage is false or stream unset, got: %s", bodyStr)
	}
}

func TestBuildBody_OpenAI_MaxCompletionTokens(t *testing.T) {
	p := Provider{Protocol: "openai_chat"}
	tgt := Target{
		ModelName:      "o1-preview",
		MaxTokensField: "max_completion_tokens",
		Core: CoreParams{
			MaxTokens: intPtr(4096),
			Stream:    boolPtr(true),
		},
	}
	in := Input{
		System: "You are a helpful assistant.",
		Messages: []Message{
			{Role: "user", Content: rawContent("hi")},
		},
	}

	body, err := BuildBody(p, tgt, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("failed to unmarshal body: %v", err)
	}

	if parsed["max_completion_tokens"] != float64(4096) {
		t.Errorf("expected max_completion_tokens 4096, got %v", parsed["max_completion_tokens"])
	}
	if _, ok := parsed["max_tokens"]; ok {
		t.Errorf("max_tokens should not be present")
	}

	if _, ok := parsed["stream_options"]; ok {
		t.Errorf("stream_options should not be present, got %v", parsed["stream_options"])
	}

	msgs, ok := parsed["messages"].([]any)
	if !ok || len(msgs) != 2 {
		t.Fatalf("expected 2 messages (system prepended), got %v", msgs)
	}
	sysMsg := msgs[0].(map[string]any)
	if sysMsg["role"] != "system" || sysMsg["content"] != "You are a helpful assistant." {
		t.Errorf("first message should be system message, got %v", sysMsg)
	}
}

func TestBuildBody_Anthropic_MissingMaxTokensError(t *testing.T) {
	p := Provider{Protocol: "anthropic"}
	tgt := Target{
		ModelName: "claude-3-5-sonnet",
		Core:      CoreParams{}, // MaxTokens is nil
	}
	in := Input{
		Messages: []Message{{Role: "user", Content: rawContent("hi")}},
	}

	_, err := BuildBody(p, tgt, in)
	if err == nil {
		t.Fatalf("expected error for missing max_tokens, got nil")
	}
	if !strings.Contains(err.Error(), "max_tokens is required for anthropic") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestBuildBody_Anthropic_SeedNotSupportedError(t *testing.T) {
	p := Provider{Protocol: "anthropic"}
	tgt := Target{
		ModelName: "claude-3-5-sonnet",
		Core: CoreParams{
			MaxTokens: intPtr(1000),
			Seed:      int64Ptr(42),
		},
	}
	in := Input{
		Messages: []Message{{Role: "user", Content: rawContent("hi")}},
	}

	_, err := BuildBody(p, tgt, in)
	if err == nil {
		t.Fatalf("expected error for seed, got nil")
	}
	if !strings.Contains(err.Error(), "seed is not supported for anthropic") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestBuildBody_Anthropic_Success(t *testing.T) {
	p := Provider{Protocol: "anthropic"}
	tgt := Target{
		ModelName: "claude-3-5-sonnet",
		Core: CoreParams{
			Temperature: floatPtr(0.7),
			MaxTokens:   intPtr(2048),
			Stop:        []string{"STOP"},
		},
	}
	in := Input{
		System: "Anthropic system prompt",
		Messages: []Message{
			{Role: "user", Content: rawContent("hello anthropic")},
		},
	}

	body, err := BuildBody(p, tgt, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	if parsed["system"] != "Anthropic system prompt" {
		t.Errorf("expected system string at top level, got %v", parsed["system"])
	}
	if parsed["max_tokens"] != float64(2048) {
		t.Errorf("expected max_tokens 2048, got %v", parsed["max_tokens"])
	}
	stops, ok := parsed["stop_sequences"].([]any)
	if !ok || len(stops) != 1 || stops[0] != "STOP" {
		t.Errorf("expected stop_sequences [\"STOP\"], got %v", parsed["stop_sequences"])
	}
}

func TestBuildBody_ExtraOverrideAndNullDelete(t *testing.T) {
	p := Provider{Protocol: "openai_chat"}
	tgt := Target{
		ModelName: "gpt-4o",
		Core: CoreParams{
			Temperature: floatPtr(0.5),
		},
		Extra: map[string]any{
			"temperature": 0.9,     // override core
			"stream":      nil,     // delete stream
			"response_format": map[string]any{
				"type": "json_object",
			},
		},
	}
	in := Input{
		Messages: []Message{{Role: "user", Content: rawContent("hi")}},
	}

	body, err := BuildBody(p, tgt, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	if parsed["temperature"] != 0.9 {
		t.Errorf("expected temperature 0.9 (extra overrides core), got %v", parsed["temperature"])
	}
	if _, exists := parsed["stream"]; exists {
		t.Errorf("expected stream key to be deleted by null extra, got %v", parsed["stream"])
	}
	rf, ok := parsed["response_format"].(map[string]any)
	if !ok || rf["type"] != "json_object" {
		t.Errorf("expected response_format type json_object, got %v", parsed["response_format"])
	}
}

func TestBuildBody_NoHTMLEscapeAndNoUnicodeEscape(t *testing.T) {
	p := Provider{Protocol: "openai_chat"}
	tgt := Target{
		ModelName: "gpt-4o",
	}
	specialText := "Hello <world> & friends! 测试中文特殊符号"
	in := Input{
		Messages: []Message{
			{Role: "user", Content: rawContent(specialText)},
		},
	}

	body, err := BuildBody(p, tgt, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	raw := string(body)
	if strings.Contains(raw, `\u003c`) || strings.Contains(raw, `\u003e`) || strings.Contains(raw, `\u0036`) {
		t.Errorf("HTML characters should not be escaped, got: %s", raw)
	}
	if !strings.Contains(raw, "<world>") {
		t.Errorf("expected literal <world> in body, got: %s", raw)
	}
	if !strings.Contains(raw, "& friends") {
		t.Errorf("expected literal & friends in body, got: %s", raw)
	}
	if !strings.Contains(raw, "测试中文特殊符号") {
		t.Errorf("expected literal Chinese in body, got: %s", raw)
	}
}
