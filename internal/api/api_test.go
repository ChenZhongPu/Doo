package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"doo/internal/secrets"
	"doo/internal/store"
)

func setupTestServer(t *testing.T) (*Server, *store.Store, *secrets.KeyStore) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}

	keyPath := filepath.Join(tmpDir, "master.key")
	ks, err := secrets.NewKeyStore(keyPath)
	if err != nil {
		t.Fatalf("failed to create keystore: %v", err)
	}

	srv := NewServer(st, ks, "1.0.0-test", "2026-10-08", "testcommit", SecurityConfig{BindAddr: "127.0.0.1:8787"}, 8)
	return srv, st, ks
}

func TestAPI_SecurityMiddleware(t *testing.T) {
	srv, st, _ := setupTestServer(t)
	defer st.Close()

	handler := srv.Handler()

	// 1. Valid Host
	req := httptest.NewRequest("GET", "http://127.0.0.1:8787/api/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for 127.0.0.1, got %d", w.Code)
	}

	// 2. Invalid Host -> 403
	reqBadHost := httptest.NewRequest("GET", "http://evil.com/api/health", nil)
	wBadHost := httptest.NewRecorder()
	handler.ServeHTTP(wBadHost, reqBadHost)
	if wBadHost.Code != http.StatusForbidden {
		t.Errorf("expected 403 for evil.com Host, got %d", wBadHost.Code)
	}

	// 3. Cross-origin POST -> 403
	body := strings.NewReader(`{"name":"test"}`)
	reqBadOrigin := httptest.NewRequest("POST", "http://127.0.0.1:8787/api/providers", body)
	reqBadOrigin.Header.Set("Origin", "http://malicious.com")
	wBadOrigin := httptest.NewRecorder()
	handler.ServeHTTP(wBadOrigin, reqBadOrigin)
	if wBadOrigin.Code != http.StatusForbidden {
		t.Errorf("expected 403 for cross-origin POST, got %d", wBadOrigin.Code)
	}
}

func TestAPI_PreviewAndValidation(t *testing.T) {
	srv, st, ks := setupTestServer(t)
	defer st.Close()

	handler := srv.Handler()

	// 1. Create Provider
	pID, err := st.CreateProvider(&store.ProviderRecord{
		Name:      "test-openai",
		Protocol:  "openai_chat",
		BaseURL:   "https://api.openai.com/v1",
		APIKeyRef: "env:OPENAI_API_KEY",
	})
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}

	// 2. Create Target
	maxTok := 100
	tID, err := st.CreateTarget(&store.TargetRecord{
		ProviderID: pID,
		Label:      "gpt-test",
		ModelName:  "gpt-4o",
		Core:       store.TargetRecord{}.Core,
	})
	if err != nil {
		t.Fatalf("create target: %v", err)
	}

	_ = maxTok
	_ = ks

	// 3. POST /api/preview
	previewPayload := fmt.Sprintf(`{
		"target_id": %d,
		"input": {
			"system": "be nice",
			"messages": [{"role": "user", "content": "hello"}]
		}
	}`, tID)
	req := httptest.NewRequest("POST", "/api/preview", strings.NewReader(previewPayload))
	req.Host = "127.0.0.1:8787"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for preview, got %d: %s", w.Code, w.Body.String())
	}

	var prevResp PreviewResponse
	if err := json.Unmarshal(w.Body.Bytes(), &prevResp); err != nil {
		t.Fatalf("unmarshal preview response: %v", err)
	}
	if !strings.Contains(prevResp.Body, `"model":"gpt-4o"`) {
		t.Errorf("preview body missing model gpt-4o: %s", prevResp.Body)
	}
	if prevResp.Headers["Authorization"] != "Bearer ***" {
		t.Errorf("preview headers authorization not masked: %v", prevResp.Headers)
	}
}

func TestAPI_Runs_ValidationFailureStopsAll(t *testing.T) {
	srv, st, _ := setupTestServer(t)
	defer st.Close()

	handler := srv.Handler()

	// Provider 1: Anthropic
	pAnthropicID, _ := st.CreateProvider(&store.ProviderRecord{
		Name:      "anthropic",
		Protocol:  "anthropic",
		BaseURL:   "https://api.anthropic.com/v1",
		APIKeyRef: "env:ANTHROPIC_KEY",
	})

	// Target 1: Anthropic but MISSING required max_tokens!
	tgtAnthropicID, _ := st.CreateTarget(&store.TargetRecord{
		ProviderID: pAnthropicID,
		Label:      "claude-invalid",
		ModelName:  "claude-3-5-sonnet",
	})

	// Provider 2: OpenAI with mock server
	var upstreamCalled bool
	mockUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer mockUpstream.Close()

	pOpenAIID, _ := st.CreateProvider(&store.ProviderRecord{
		Name:      "mock-openai",
		Protocol:  "openai_chat",
		BaseURL:   mockUpstream.URL,
		APIKeyRef: "env:TEST_KEY",
	})
	tgtOpenAIID, _ := st.CreateTarget(&store.TargetRecord{
		ProviderID: pOpenAIID,
		Label:      "gpt-mock",
		ModelName:  "gpt-4o",
	})

	t.Setenv("ANTHROPIC_KEY", "claude-secret")
	t.Setenv("TEST_KEY", "openai-secret")

	// Run both targets. Since Anthropic target is missing max_tokens, run should fail with 400 and NOT call upstream!
	runPayload := fmt.Sprintf(`{
		"target_ids": [%d, %d],
		"input": {
			"messages": [{"role": "user", "content": "hi"}]
		}
	}`, tgtOpenAIID, tgtAnthropicID)

	req := httptest.NewRequest("POST", "/api/runs", strings.NewReader(runPayload))
	req.Host = "127.0.0.1:8787"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for validation failure, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "claude-invalid") || !strings.Contains(w.Body.String(), "max_tokens is required") {
		t.Errorf("expected error message to identify claude-invalid, got: %s", w.Body.String())
	}
	if upstreamCalled {
		t.Errorf("upstream should NOT have been called when any target validation fails!")
	}
}

func TestAPI_Runs_SSEStreamAndExport(t *testing.T) {
	srv, st, _ := setupTestServer(t)
	defer st.Close()

	// Upstream mock server
	upstreamBodies := make(chan string, 1)
	mockUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		upstreamBodies <- string(body)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Hello world!\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":5,\"completion_tokens\":10}}\n\n")
		flusher.Flush()
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer mockUpstream.Close()

	srv.HTTPClient = mockUpstream.Client()

	pID, _ := st.CreateProvider(&store.ProviderRecord{
		Name:        "openai-mock",
		Protocol:    "openai_chat",
		BaseURL:     mockUpstream.URL,
		APIKeyRef:   "env:MOCK_KEY",
		StreamUsage: true, // Previously saved providers must also stop injecting options.
	})
	tgtID, _ := st.CreateTarget(&store.TargetRecord{
		ProviderID: pID,
		Label:      "gpt-mock",
		ModelName:  "gpt-4o",
	})

	t.Setenv("MOCK_KEY", "sk-mock-12345")
	previewReq := httptest.NewRequest("POST", "/api/preview", strings.NewReader(fmt.Sprintf(`{"target_id":%d,"input":{"messages":[{"role":"user","content":"hi"}]}}`, tgtID)))
	previewReq.Host = "127.0.0.1:8787"
	previewW := httptest.NewRecorder()
	srv.Handler().ServeHTTP(previewW, previewReq)
	if previewW.Code != http.StatusOK {
		t.Fatalf("preview failed: %s", previewW.Body.String())
	}
	var preview PreviewResponse
	if err := json.Unmarshal(previewW.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(preview.Body, "stream_options") {
		t.Fatalf("preview includes automatic options: %s", preview.Body)
	}

	runPayload := fmt.Sprintf(`{
		"target_ids": [%d],
		"input": {
			"messages": [{"role": "user", "content": "hi"}]
		}
	}`, tgtID)

	req := httptest.NewRequest("POST", "/api/runs", strings.NewReader(runPayload))
	req.Host = "127.0.0.1:8787"
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	select {
	case upstreamBody := <-upstreamBodies:
		if upstreamBody != preview.Body {
			t.Fatalf("actual request differs from preview: %s vs %s", upstreamBody, preview.Body)
		}
	default:
		t.Fatal("upstream request was not sent")
	}

	body := w.Body.String()
	if !strings.Contains(body, `"type":"run"`) {
		t.Errorf("missing run event in SSE stream: %s", body)
	}
	if !strings.Contains(body, `"type":"delta"`) {
		t.Errorf("missing delta event in SSE stream: %s", body)
	}
	if !strings.Contains(body, `"type":"done"`) {
		t.Errorf("missing done event in SSE stream: %s", body)
	}
	if !strings.Contains(body, `"type":"end"`) {
		t.Errorf("missing end event in SSE stream: %s", body)
	}

	// Verify run recorded in store
	runs, err := st.ListRuns(10, 0)
	if err != nil || len(runs) != 1 {
		t.Fatalf("expected 1 run in store, got %d", len(runs))
	}
	callID := runs[0].Calls[0].ID

	// Test GET /api/calls/{id}/raw
	rawReq := httptest.NewRequest("GET", fmt.Sprintf("/api/calls/%d/raw", callID), nil)
	rawReq.Host = "127.0.0.1:8787"
	rawW := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rawW, rawReq)

	if rawW.Code != http.StatusOK {
		t.Fatalf("expected 200 for raw call, got %d: %s", rawW.Code, rawW.Body.String())
	}
	var callRaw store.CallRaw
	_ = json.Unmarshal(rawW.Body.Bytes(), &callRaw)
	if !strings.Contains(callRaw.ResponseRaw, "Hello world!") {
		t.Errorf("response raw missing content, got: %s", callRaw.ResponseRaw)
	}
	if callRaw.RequestJSON != preview.Body {
		t.Fatalf("saved request differs from preview: %s", callRaw.RequestJSON)
	}
	detail, err := st.GetRun(runs[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Calls[0].PromptTokens == nil || *detail.Calls[0].PromptTokens != 5 || detail.Calls[0].CompletionTokens == nil || *detail.Calls[0].CompletionTokens != 10 {
		t.Fatal("upstream usage was not recorded")
	}

	// Test GET /api/calls/{id}/export?format=curl
	exportReq := httptest.NewRequest("GET", fmt.Sprintf("/api/calls/%d/export?format=curl", callID), nil)
	exportReq.Host = "127.0.0.1:8787"
	exportW := httptest.NewRecorder()
	srv.Handler().ServeHTTP(exportW, exportReq)

	if exportW.Code != http.StatusOK {
		t.Fatalf("expected 200 for export curl, got %d", exportW.Code)
	}
	if !strings.Contains(exportW.Body.String(), "curl -N -X POST") {
		t.Errorf("curl export mismatch: %s", exportW.Body.String())
	}
	if strings.Contains(exportW.Body.String(), "stream_options") {
		t.Fatal("export includes automatic stream_options")
	}
}

func parseSSEEvents(raw string) []map[string]any {
	var events []map[string]any
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			var ev map[string]any
			if err := json.Unmarshal([]byte(line[6:]), &ev); err == nil {
				events = append(events, ev)
			}
		}
	}
	return events
}

func TestAPI_FetchProviderModels(t *testing.T) {
	// Mock upstream /v1/models server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		resp := map[string]any{
			"object": "list",
			"data": []map[string]any{
				{"id": "meta-llama-3-8b-instruct", "object": "model"},
				{"id": "qwen2.5-7b-instruct", "object": "model"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	srv, st, ks := setupTestServer(t)
	defer st.Close()

	encKey, err := ks.Encrypt("test-key")
	if err != nil {
		t.Fatalf("encrypt key: %v", err)
	}

	// 1. Create provider pointing to mockServer
	pID, err := st.CreateProvider(&store.ProviderRecord{
		Name:      "test-lmstudio",
		Protocol:  "openai_chat",
		BaseURL:   mockServer.URL + "/v1",
		APIKeyRef: encKey,
	})
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}

	// 2. Fetch models via API
	req := httptest.NewRequest("GET", fmt.Sprintf("/api/providers/%d/models", pID), nil)
	req.Host = "127.0.0.1:8787"
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}

	var res struct {
		Models []string `json:"models"`
		Count  int      `json:"count"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if res.Count != 2 || len(res.Models) != 2 {
		t.Fatalf("expected 2 models, got %d, body: %s", res.Count, w.Body.String())
	}
	if res.Models[0] != "meta-llama-3-8b-instruct" || res.Models[1] != "qwen2.5-7b-instruct" {
		t.Errorf("unexpected models: %v", res.Models)
	}
}
