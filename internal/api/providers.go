package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"doo/internal/adapter"
	"doo/internal/secrets"
	"doo/internal/store"
)

type ProviderPayload struct {
	Name        string            `json:"name"`
	Protocol    string            `json:"protocol"`
	BaseURL     string            `json:"base_url"`
	APIKeyRef   string            `json:"api_key_ref"`
	APIKey      string            `json:"api_key,omitempty"` // For input only, never stored in plaintext
	Headers     map[string]string `json:"headers"`
	StreamUsage bool              `json:"stream_usage"`
}

type ProviderResponse struct {
	ID          int64             `json:"id"`
	Name        string            `json:"name"`
	Protocol    string            `json:"protocol"`
	BaseURL     string            `json:"base_url"`
	APIKeyRef   string            `json:"api_key_ref"`
	Headers     map[string]string `json:"headers"`
	StreamUsage bool              `json:"stream_usage"`
	CreatedAt   int64             `json:"created_at"`
	UpdatedAt   int64             `json:"updated_at"`
}

func toProviderResponse(p *store.ProviderRecord) ProviderResponse {
	return ProviderResponse{
		ID:          p.ID,
		Name:        p.Name,
		Protocol:    p.Protocol,
		BaseURL:     p.BaseURL,
		APIKeyRef:   secrets.MaskRef(p.APIKeyRef),
		Headers:     p.Headers,
		StreamUsage: p.StreamUsage,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
}

func (s *Server) handleListProviders(w http.ResponseWriter, r *http.Request) {
	list, err := s.Store.ListProviders()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	res := make([]ProviderResponse, 0, len(list))
	for _, p := range list {
		res = append(res, toProviderResponse(p))
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleCreateProvider(w http.ResponseWriter, r *http.Request) {
	var payload ProviderPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	payload.Name = strings.TrimSpace(payload.Name)
	payload.BaseURL = strings.TrimSpace(payload.BaseURL)
	payload.Protocol = strings.TrimSpace(payload.Protocol)

	if payload.Name == "" || payload.BaseURL == "" {
		writeError(w, http.StatusBadRequest, "name and base_url are required")
		return
	}
	if payload.Protocol != "openai_chat" && payload.Protocol != "anthropic" {
		writeError(w, http.StatusBadRequest, "protocol must be openai_chat or anthropic")
		return
	}

	keyRef := strings.TrimSpace(payload.APIKeyRef)
	if payload.APIKey != "" {
		enc, err := s.KeyStore.Encrypt(payload.APIKey)
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("encrypt key: %v", err))
			return
		}
		keyRef = enc
	} else if keyRef == "" {
		writeError(w, http.StatusBadRequest, "api_key or api_key_ref is required")
		return
	}

	if payload.Headers == nil {
		payload.Headers = make(map[string]string)
	}

	rec := &store.ProviderRecord{
		Name:        payload.Name,
		Protocol:    payload.Protocol,
		BaseURL:     payload.BaseURL,
		APIKeyRef:   keyRef,
		Headers:     payload.Headers,
		StreamUsage: payload.StreamUsage,
	}

	id, err := s.Store.CreateProvider(rec)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("A provider named %q already exists", payload.Name))
			return
		}
		writeError(w, http.StatusBadRequest, fmt.Sprintf("create provider: %v", err))
		return
	}
	rec.ID = id

	writeJSON(w, http.StatusCreated, toProviderResponse(rec))
}

func (s *Server) handleGetProvider(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid provider id")
		return
	}

	p, err := s.Store.GetProvider(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if p == nil {
		writeError(w, http.StatusNotFound, "provider not found")
		return
	}

	writeJSON(w, http.StatusOK, toProviderResponse(p))
}

func (s *Server) handleUpdateProvider(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid provider id")
		return
	}

	existing, err := s.Store.GetProvider(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "provider not found")
		return
	}

	var payload ProviderPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	payload.Name = strings.TrimSpace(payload.Name)
	payload.BaseURL = strings.TrimSpace(payload.BaseURL)
	payload.Protocol = strings.TrimSpace(payload.Protocol)

	if payload.Name != "" {
		existing.Name = payload.Name
	}
	if payload.BaseURL != "" {
		existing.BaseURL = payload.BaseURL
	}
	if payload.Protocol != "" {
		if payload.Protocol != "openai_chat" && payload.Protocol != "anthropic" {
			writeError(w, http.StatusBadRequest, "protocol must be openai_chat or anthropic")
			return
		}
		existing.Protocol = payload.Protocol
	}

	if payload.APIKey != "" {
		enc, err := s.KeyStore.Encrypt(payload.APIKey)
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("encrypt key: %v", err))
			return
		}
		existing.APIKeyRef = enc
	} else if payload.APIKeyRef != "" && payload.APIKeyRef != "enc:***" {
		existing.APIKeyRef = payload.APIKeyRef
	}

	if payload.Headers != nil {
		existing.Headers = payload.Headers
	}
	existing.StreamUsage = payload.StreamUsage

	if err := s.Store.UpdateProvider(existing); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("A provider named %q already exists", payload.Name))
			return
		}
		writeError(w, http.StatusBadRequest, fmt.Sprintf("update provider: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, toProviderResponse(existing))
}

func (s *Server) handleDeleteProvider(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid provider id")
		return
	}

	if err := s.Store.DeleteProvider(id); err != nil {
		writeError(w, http.StatusNotFound, "provider not found or could not be deleted")
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleTestProvider(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid provider id")
		return
	}

	pRec, err := s.Store.GetProvider(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if pRec == nil {
		writeError(w, http.StatusNotFound, "provider not found")
		return
	}

	var testBody struct {
		ModelName string `json:"model_name"`
	}
	_ = json.NewDecoder(r.Body).Decode(&testBody)

	modelName := testBody.ModelName
	if modelName == "" {
		// Try to find first target model name for this provider
		targets, _ := s.Store.ListTargets()
		for _, tgt := range targets {
			if tgt.ProviderID == pRec.ID && tgt.ModelName != "" {
				modelName = tgt.ModelName
				break
			}
		}
	}
	if modelName == "" {
		if pRec.Protocol == "anthropic" {
			modelName = "claude-3-haiku-20240307"
		} else {
			modelName = "gpt-4o-mini"
		}
	}

	apiKey, err := s.KeyStore.Resolve(pRec.APIKeyRef)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":          false,
			"http_status": 0,
			"total_ms":    0,
			"error":       fmt.Sprintf("resolve key: %v", err),
		})
		return
	}

	prov := adapter.Provider{
		ID:          pRec.ID,
		Name:        pRec.Name,
		Protocol:    pRec.Protocol,
		BaseURL:     pRec.BaseURL,
		APIKeyRef:   pRec.APIKeyRef,
		Headers:     pRec.Headers,
		StreamUsage: pRec.StreamUsage,
	}

	maxTok := 8
	tgt := adapter.Target{
		ModelName: modelName,
		Core: adapter.CoreParams{
			MaxTokens: &maxTok,
		},
	}

	in := adapter.Input{
		Messages: []adapter.Message{
			{Role: "user", Content: json.RawMessage(`"ping"`)},
		},
	}

	reqBody, err := adapter.BuildBody(prov, tgt, in)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":          false,
			"http_status": 0,
			"total_ms":    0,
			"error":       fmt.Sprintf("build test body: %v", err),
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var ad adapter.Adapter
	if prov.Protocol == "anthropic" {
		ad = &adapter.AnthropicAdapter{Client: s.HTTPClient, Clock: s.Clock}
	} else {
		ad = &adapter.OpenAIChatAdapter{Client: s.HTTPClient, Clock: s.Clock}
	}

	res := ad.Do(ctx, prov, apiKey, reqBody, nil)

	tot := 0.0
	if res.Metrics.TotalMs != nil {
		tot = *res.Metrics.TotalMs
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":          res.Status == "done",
		"http_status": res.HTTPStatus,
		"total_ms":    tot,
		"error":       res.Err,
	})
}

func (s *Server) handleFetchProviderModels(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid provider id")
		return
	}

	pRec, err := s.Store.GetProvider(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if pRec == nil {
		writeError(w, http.StatusNotFound, "provider not found")
		return
	}

	baseURL := strings.TrimRight(strings.TrimSpace(pRec.BaseURL), "/")
	if baseURL == "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":     false,
			"error":  "base_url is empty",
			"models": []string{},
		})
		return
	}

	modelsURL := baseURL + "/models"
	if !strings.HasSuffix(baseURL, "/v1") && !strings.Contains(baseURL, "/v1/") {
		modelsURL = baseURL + "/v1/models"
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, "GET", modelsURL, nil)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":     false,
			"error":  fmt.Sprintf("create request: %v", err),
			"models": []string{},
		})
		return
	}

	// Apply headers
	for k, v := range pRec.Headers {
		httpReq.Header.Set(k, v)
	}

	apiKey, _ := s.KeyStore.Resolve(pRec.APIKeyRef)
	if apiKey != "" && apiKey != "none" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := s.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":     false,
			"error":  fmt.Sprintf("Failed to connect to %s: %v", modelsURL, err),
			"models": []string{},
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":          false,
			"http_status": resp.StatusCode,
			"error":       fmt.Sprintf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes))),
			"models":      []string{},
		})
		return
	}

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":     false,
			"error":  fmt.Sprintf("read response: %v", err),
			"models": []string{},
		})
		return
	}

	var parsed any
	if err := json.Unmarshal(bodyBytes, &parsed); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":     false,
			"error":  fmt.Sprintf("parse json: %v", err),
			"models": []string{},
		})
		return
	}

	modelList := extractModelNames(parsed)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":          true,
		"http_status": resp.StatusCode,
		"models":      modelList,
		"count":       len(modelList),
	})
}

func extractModelNames(v any) []string {
	results := make([]string, 0)
	seen := make(map[string]bool)

	add := func(name string) {
		name = strings.TrimSpace(name)
		if name != "" && !seen[name] {
			seen[name] = true
			results = append(results, name)
		}
	}

	switch val := v.(type) {
	case []any:
		for _, item := range val {
			if str, ok := item.(string); ok {
				add(str)
			} else if m, ok := item.(map[string]any); ok {
				if id, ok := m["id"].(string); ok {
					add(id)
				} else if name, ok := m["name"].(string); ok {
					add(name)
				} else if model, ok := m["model"].(string); ok {
					add(model)
				}
			}
		}
	case map[string]any:
		// Check "data" field (standard OpenAI / LM Studio / Ollama)
		if data, ok := val["data"].([]any); ok {
			for _, item := range data {
				if str, ok := item.(string); ok {
					add(str)
				} else if m, ok := item.(map[string]any); ok {
					if id, ok := m["id"].(string); ok {
						add(id)
					} else if name, ok := m["name"].(string); ok {
						add(name)
					}
				}
			}
		}
		// Check "models" field (Ollama native)
		if models, ok := val["models"].([]any); ok {
			for _, item := range models {
				if str, ok := item.(string); ok {
					add(str)
				} else if m, ok := item.(map[string]any); ok {
					if name, ok := m["name"].(string); ok {
						add(name)
					} else if model, ok := m["model"].(string); ok {
						add(model)
					} else if id, ok := m["id"].(string); ok {
						add(id)
					}
				}
			}
		}
	}

	sort.Strings(results)
	return results
}

