package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"doo/internal/store"
)

type connectionTestTransport func(*http.Request) (*http.Response, error)

func (f connectionTestTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func testConnection(t *testing.T, s *Server, id int64, body string, ctx context.Context) providerTestResult {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/providers/%d/test", id), strings.NewReader(body))
	r.Host = "127.0.0.1:8787"
	if ctx != nil {
		r = r.WithContext(ctx)
	}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("HTTP %d: %s", w.Code, w.Body.String())
	}
	var result providerTestResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestProviderConnectionQueries(t *testing.T) {
	tests := []struct {
		name, protocol, base, wantURL, response string
	}{
		{"renamed OpenAI", "openai_chat", "https://api.openai.com/v1/", "https://api.openai.com/v1/models", `{"data":[]}`},
		{"Anthropic", "anthropic", "https://api.anthropic.com/v1", "https://api.anthropic.com/v1/models", `{"data":[]}`},
		{"DeepSeek", "openai_chat", "https://api.deepseek.com", "https://api.deepseek.com/user/balance", `{"is_available":false,"balance_infos":[]}`},
		{"OpenRouter", "openai_chat", "https://openrouter.ai/api/v1", "https://openrouter.ai/api/v1/key", `{"data":{"label":"test","limit_remaining":0}}`},
		{"SiliconFlow", "openai_chat", "https://api.siliconflow.cn/v1", "https://api.siliconflow.cn/v1/models", `{"data":[]}`},
		{"SiliconFlow global", "openai_chat", "https://api.siliconflow.com/v1", "https://api.siliconflow.com/v1/models", `{"data":[]}`},
		{"Groq", "openai_chat", "https://api.groq.com/openai/v1", "https://api.groq.com/openai/v1/models", `{"data":[]}`},
		{"Kimi", "openai_chat", "https://api.moonshot.cn/v1", "https://api.moonshot.cn/v1/users/me/balance", `{"status":true,"data":{"available_balance":0}}`},
		{"Kimi global", "openai_chat", "https://api.moonshot.ai/v1", "https://api.moonshot.ai/v1/users/me/balance", `{"status":true,"data":{"available_balance":1}}`},
		{"Qwen Singapore", "openai_chat", "https://dashscope-intl.aliyuncs.com/compatible-mode/v1", "https://dashscope-intl.aliyuncs.com/api/v1/models?page_size=1", `{"success":true,"output":{"models":[]}}`},
		{"Qwen Beijing", "openai_chat", "https://workspace.cn-beijing.maas.aliyuncs.com/api/v1", "https://workspace.cn-beijing.maas.aliyuncs.com/api/v1/models?page_size=1", `{"output":{"models":[]}}`},
		{"Qwen Hong Kong", "openai_chat", "https://cn-hongkong.dashscope.aliyuncs.com/compatible-mode/v1", "https://cn-hongkong.dashscope.aliyuncs.com/api/v1/models?page_size=1", `{"output":{"models":[]}}`},
		{"local server", "openai_chat", "http://localhost:1234/v1", "http://localhost:1234/v1/models", `{"data":[]}`},
		{"LM Studio LAN", "openai_chat", "http://192.168.1.2:1234/v1", "http://192.168.1.2:1234/v1/models", `{"data":[]}`},
		{"local server", "openai_chat", "http://[::1]:11434/v1", "http://[::1]:11434/api/version", `{"version":"0.12.0"}`},
		{"Ollama LAN", "openai_chat", "http://192.168.1.2:11434/proxy/v1", "http://192.168.1.2:11434/proxy/api/version", `{"version":"0.12.0"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name+tt.base, func(t *testing.T) {
			s, st, _ := setupTestServer(t)
			defer st.Close()
			t.Setenv("PROBE_TEST_KEY", "query-secret")
			p := &store.ProviderRecord{Name: tt.name, Protocol: tt.protocol, BaseURL: tt.base, APIKeyRef: "env:PROBE_TEST_KEY", Headers: map[string]string{"X-Custom": "kept"}}
			id, err := st.CreateProvider(p)
			if err != nil {
				t.Fatal(err)
			}
			if toProviderResponse(p).TestMode != "query" {
				t.Fatal("UI metadata must allow testing without a model")
			}
			calls := 0
			s.HTTPClient = &http.Client{Transport: connectionTestTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.Method != http.MethodGet || r.URL.String() != tt.wantURL || r.Body != nil {
					t.Fatalf("unexpected inference/request: %s %s body=%v", r.Method, r.URL, r.Body)
				}
				if r.Header.Get("X-Custom") != "kept" {
					t.Fatal("custom header lost")
				}
				if tt.protocol == "anthropic" {
					if r.Header.Get("x-api-key") != "query-secret" || r.Header.Get("anthropic-version") != "2023-06-01" {
						t.Fatal("missing Anthropic authentication")
					}
				} else if r.Header.Get("Authorization") != "Bearer query-secret" {
					t.Fatal("missing Bearer authentication")
				}
				deadline, ok := r.Context().Deadline()
				if !ok || time.Until(deadline) > 10*time.Second {
					t.Fatal("query must have a bounded timeout")
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(tt.response)), Header: make(http.Header)}, nil
			})}
			result := testConnection(t, s, id, `{}`, nil)
			if !result.OK || !result.Reachable || result.Mode != "query" || result.State != "connected" || calls != 1 {
				t.Fatalf("unexpected result: %+v, calls=%d", result, calls)
			}
		})
	}
}

func TestProviderConnectionRequiresConfiguredModel(t *testing.T) {
	for _, p := range []*store.ProviderRecord{
		{Name: "GLM", Protocol: "openai_chat", BaseURL: "https://open.bigmodel.cn/api/paas/v4"},
		{Name: "Qwen", Protocol: "openai_chat", BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1"},
		{Name: "OpenAI proxy", Protocol: "openai_chat", BaseURL: "https://gateway.example/v1"},
		{Name: "custom Anthropic", Protocol: "anthropic", BaseURL: "https://gateway.example/v1"},
		{Name: "custom", Protocol: "openai_chat", BaseURL: "http://localhost:8080/v1"},
		{Name: "custom", Protocol: "openai_chat", BaseURL: "https://api.openai.com.example/v1"},
	} {
		t.Run(p.Name+p.BaseURL, func(t *testing.T) {
			s, st, _ := setupTestServer(t)
			defer st.Close()
			p.APIKeyRef = "env:UNSET_CONNECTION_KEY"
			id, err := st.CreateProvider(p)
			if err != nil {
				t.Fatal(err)
			}
			if toProviderResponse(p).TestMode != "ping" {
				t.Fatal("unknown provider must require a model")
			}
			s.HTTPClient = &http.Client{Transport: connectionTestTransport(func(r *http.Request) (*http.Response, error) {
				t.Fatal("must not contact the provider without a configured model")
				return nil, nil
			})}
			// A caller cannot bypass configuration by supplying an arbitrary model.
			result := testConnection(t, s, id, `{"model_name":"gpt-4o-mini"}`, nil)
			if result.OK || result.State != "model_required" || result.Mode != "ping" {
				t.Fatalf("unexpected result: %+v", result)
			}
		})
	}
}

func TestProviderConnectionPingUsesFirstConfiguredModel(t *testing.T) {
	for _, protocol := range []string{"openai_chat", "anthropic"} {
		t.Run(protocol, func(t *testing.T) {
			s, st, _ := setupTestServer(t)
			defer st.Close()
			t.Setenv("PING_TEST_KEY", "ping-secret")
			id, err := st.CreateProvider(&store.ProviderRecord{Name: "Custom", Protocol: protocol, BaseURL: "https://gateway.example/v1", APIKeyRef: "env:PING_TEST_KEY"})
			if err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"configured-first", "configured-second"} {
				if _, err := st.CreateTarget(&store.TargetRecord{ProviderID: id, ModelName: name, Label: name, MaxTokensField: "max_completion_tokens"}); err != nil {
					t.Fatal(err)
				}
			}
			s.HTTPClient = &http.Client{Transport: connectionTestTransport(func(r *http.Request) (*http.Response, error) {
				var payload map[string]any
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Fatal(err)
				}
				if r.Method != http.MethodPost || payload["model"] != "configured-first" || payload["stream"] != true || payload["stream_options"] != nil {
					t.Fatalf("unexpected ping: %v", payload)
				}
				messages, _ := payload["messages"].([]any)
				if len(messages) != 1 || messages[0].(map[string]any)["content"] != "ping" {
					t.Fatalf("expected ping message: %v", payload)
				}
				response := "data: {\"choices\":[{\"delta\":{\"content\":\"pong\"},\"finish_reason\":null}]}\n\ndata: [DONE]\n\n"
				if protocol == "anthropic" {
					if !strings.HasSuffix(r.URL.Path, "/messages") || payload["max_tokens"] != float64(8) {
						t.Fatalf("unexpected Anthropic ping: %s %v", r.URL, payload)
					}
					response = "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"pong\"}}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
				} else if !strings.HasSuffix(r.URL.Path, "/chat/completions") || payload["max_completion_tokens"] != float64(8) {
					t.Fatalf("unexpected OpenAI ping: %s %v", r.URL, payload)
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(response)), Header: http.Header{"Content-Type": {"text/event-stream"}}}, nil
			})}
			result := testConnection(t, s, id, `{}`, nil)
			if !result.OK || result.Mode != "ping" || result.ModelName != "configured-first" {
				t.Fatalf("unexpected result: %+v", result)
			}
			// Neither the ping nor a query should create run history.
			r := httptest.NewRequest(http.MethodGet, "/api/runs", nil)
			r.Host = "127.0.0.1:8787"
			w := httptest.NewRecorder()
			s.Handler().ServeHTTP(w, r)
			var runs []any
			if json.Unmarshal(w.Body.Bytes(), &runs) != nil || len(runs) != 0 {
				t.Fatalf("test generated history: %s", w.Body.String())
			}
		})
	}
}

func TestProviderConnectionQueryFailures(t *testing.T) {
	t.Setenv("CONNECTION_FAILURE_KEY", "test-key")
	for _, tt := range []struct {
		name, body, state string
		status            int
		err               error
	}{
		{"invalid key", `{"error":"invalid key"}`, "auth_error", 401, nil},
		{"permissions", `{"error":"forbidden"}`, "auth_error", 403, nil},
		{"missing endpoint", "not found", "unsupported", 404, nil},
		{"method", "method not allowed", "unsupported", 405, nil},
		{"rate limit", "rate limited", "rate_limited", 429, nil},
		{"service unavailable", "unavailable", "service_error", 503, nil},
		{"HTML success", "<html>login</html>", "invalid_response", 200, nil},
		{"error in JSON", `{"error":{"message":"bad key"},"data":[]}`, "invalid_response", 200, nil},
		{"wrong JSON shape", `{"ok":true}`, "invalid_response", 200, nil},
		{"DNS failure", "", "connection_error", 0, errors.New("DNS lookup failed")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s, st, _ := setupTestServer(t)
			defer st.Close()
			id, err := st.CreateProvider(&store.ProviderRecord{Name: "OpenAI", Protocol: "openai_chat", BaseURL: "https://api.openai.com/v1", APIKeyRef: "env:CONNECTION_FAILURE_KEY"})
			if err != nil {
				t.Fatal(err)
			}
			// Even with a model configured, failed queries must not fall back to inference.
			st.CreateTarget(&store.TargetRecord{ProviderID: id, ModelName: "never-use", Label: "never-use"})
			calls := 0
			s.HTTPClient = &http.Client{Transport: connectionTestTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.Method != http.MethodGet {
					t.Fatal("query failure triggered inference")
				}
				if tt.err != nil {
					return nil, tt.err
				}
				return &http.Response{StatusCode: tt.status, Body: io.NopCloser(strings.NewReader(tt.body)), Header: make(http.Header)}, nil
			})}
			result := testConnection(t, s, id, `{}`, nil)
			if result.OK || result.State != tt.state || result.Reachable != (tt.err == nil) || result.Error == "" || calls != 1 {
				t.Fatalf("unexpected result: %+v, calls=%d", result, calls)
			}
		})
	}
}

func TestProviderConnectionHonorsCancellationAndHeaderOverrides(t *testing.T) {
	s, st, _ := setupTestServer(t)
	defer st.Close()
	t.Setenv("PROBE_OVERRIDE_KEY", "stored-key")
	id, err := st.CreateProvider(&store.ProviderRecord{Name: "OpenAI", Protocol: "openai_chat", BaseURL: "https://api.openai.com/v1", APIKeyRef: "env:PROBE_OVERRIDE_KEY", Headers: map[string]string{"authorization": "Bearer override"}})
	if err != nil {
		t.Fatal(err)
	}
	s.HTTPClient = &http.Client{Transport: connectionTestTransport(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "Bearer override" {
			t.Fatal("query must honor the same custom headers as model calls")
		}
		return nil, r.Context().Err()
	})}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := testConnection(t, s, id, `{}`, ctx)
	if result.OK || result.State != "connection_error" || !strings.Contains(result.Error, "canceled") {
		t.Fatalf("unexpected canceled result: %+v", result)
	}
}
