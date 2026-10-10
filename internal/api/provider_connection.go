package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"doo/internal/adapter"
	"doo/internal/store"
)

type providerProbe struct {
	URL  string
	Kind string
}

// Only documented provider endpoints and identifiable local servers use queries.
// Unknown gateways require a configured model instead of guessing their API.
func connectionProbe(p *store.ProviderRecord) *providerProbe {
	u, err := url.Parse(strings.TrimSpace(p.BaseURL))
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil
	}
	host := strings.ToLower(u.Hostname())
	path := strings.TrimRight(u.Path, "/")
	kind := "models"
	if p.Protocol == "anthropic" {
		if host != "api.anthropic.com" {
			return nil
		}
		path += "/models"
	} else if p.Protocol == "openai_chat" {
		switch host {
		case "api.openai.com", "api.siliconflow.cn", "api.siliconflow.com", "api.groq.com":
			path += "/models"
		case "api.deepseek.com":
			path += "/user/balance"
			kind = "deepseek_balance"
		case "openrouter.ai":
			path += "/key"
			kind = "key"
		case "api.moonshot.cn", "api.moonshot.ai":
			path += "/users/me/balance"
			kind = "moonshot_balance"
		case "dashscope-intl.aliyuncs.com", "cn-hongkong.dashscope.aliyuncs.com":
			path = "/api/v1/models"
			kind = "dashscope_models"
		default:
			if strings.HasSuffix(host, ".maas.aliyuncs.com") {
				path = "/api/v1/models"
				kind = "dashscope_models"
			} else {
				name := strings.ToLower(strings.ReplaceAll(p.Name, " ", ""))
				ip := net.ParseIP(host)
				loopback := host == "localhost" || (ip != nil && ip.IsLoopback())
				switch {
				case strings.Contains(name, "ollama") || (loopback && u.Port() == "11434"):
					path = strings.TrimSuffix(path, "/v1") + "/api/version"
					kind = "version"
				case strings.Contains(name, "lmstudio") || (loopback && u.Port() == "1234"):
					path += "/models"
				default:
					return nil
				}
			}
		}
	} else {
		return nil
	}
	u.Path, u.RawPath, u.Fragment = path, "", ""
	if kind == "dashscope_models" {
		query := u.Query()
		query.Set("page_size", "1")
		u.RawQuery = query.Encode()
	}
	return &providerProbe{URL: u.String(), Kind: kind}
}

func providerTestMode(p *store.ProviderRecord) string {
	if connectionProbe(p) != nil {
		return "query"
	}
	return "ping"
}

type providerTestResult struct {
	OK         bool    `json:"ok"`
	HTTPStatus int     `json:"http_status"`
	TotalMs    float64 `json:"total_ms"`
	Mode       string  `json:"mode"`
	State      string  `json:"state"`
	Reachable  bool    `json:"reachable"`
	ModelName  string  `json:"model_name,omitempty"`
	Error      string  `json:"error,omitempty"`
}

func (s *Server) handleTestProvider(w http.ResponseWriter, r *http.Request) {
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
	probe := connectionProbe(p)
	var target *store.TargetRecord
	if probe == nil {
		var payload struct {
			ModelName string `json:"model_name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err != io.EOF {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		targets, err := s.Store.ListTargets()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		for _, candidate := range targets {
			if candidate.ProviderID == p.ID && strings.TrimSpace(candidate.ModelName) != "" &&
				(payload.ModelName == "" || candidate.ModelName == payload.ModelName) {
				target = candidate
				break
			}
		}
		if target == nil {
			writeJSON(w, http.StatusOK, providerTestResult{Mode: "ping", State: "model_required", Error: "Configure a model under this provider before testing."})
			return
		}
	}
	key, err := s.KeyStore.Resolve(p.APIKeyRef)
	if err != nil {
		writeJSON(w, http.StatusOK, providerTestResult{Mode: providerTestMode(p), State: "key_error", Error: fmt.Sprintf("resolve key: %v", err)})
		return
	}
	if probe != nil {
		writeJSON(w, http.StatusOK, s.queryProvider(r.Context(), p, key, probe))
		return
	}
	writeJSON(w, http.StatusOK, s.pingProvider(r.Context(), p, key, target))
}

func (s *Server) queryProvider(parent context.Context, p *store.ProviderRecord, key string, probe *providerProbe) providerTestResult {
	result := providerTestResult{Mode: "query", State: "connection_error"}
	start := time.Now()
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, probe.URL, nil)
	if err != nil {
		result.Error = fmt.Sprintf("create request: %v", err)
		return result
	}
	req.Header.Set("Accept", "application/json")
	if p.Protocol == "anthropic" {
		req.Header.Set("anthropic-version", "2023-06-01")
		if key != "" && key != "none" {
			req.Header.Set("x-api-key", key)
		}
	} else if key != "" && key != "none" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	for k, v := range p.Headers {
		req.Header.Set(k, v)
	}
	client := s.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	result.TotalMs = float64(time.Since(start).Microseconds()) / 1000
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer resp.Body.Close()
	result.Reachable, result.HTTPStatus = true, resp.StatusCode
	const maxBody = 2 * 1024 * 1024
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	result.TotalMs = float64(time.Since(start).Microseconds()) / 1000
	if err != nil {
		result.Error = fmt.Sprintf("read response: %v", err)
		return result
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		result.State = connectionHTTPState(resp.StatusCode)
		if len(body) > 1024 {
			body = body[:1024]
		}
		result.Error = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		return result
	}
	result.State = "invalid_response"
	if len(body) > maxBody {
		result.Error = "Response exceeds the test size limit."
		return result
	}
	if !validProbeResponse(probe.Kind, body) {
		result.Error = "The service returned an unexpected response to the test query."
		return result
	}
	result.OK, result.State = true, "connected"
	return result
}

func connectionHTTPState(status int) string {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return "auth_error"
	case http.StatusNotFound, http.StatusMethodNotAllowed:
		return "unsupported"
	case http.StatusTooManyRequests:
		return "rate_limited"
	default:
		return "service_error"
	}
}

func validProbeResponse(kind string, body []byte) bool {
	var value map[string]any
	if json.Unmarshal(body, &value) != nil || value == nil || value["error"] != nil {
		return false
	}
	if value["success"] == false || value["status"] == false {
		return false
	}
	switch kind {
	case "models":
		_, ok := value["data"].([]any)
		return ok
	case "version":
		version, ok := value["version"].(string)
		return ok && version != ""
	case "deepseek_balance":
		_, ok := value["is_available"].(bool)
		return ok
	case "moonshot_balance":
		data, ok := value["data"].(map[string]any)
		if !ok {
			return false
		}
		_, ok = data["available_balance"].(float64)
		return ok
	case "key":
		data, ok := value["data"].(map[string]any)
		return ok && len(data) > 0
	case "dashscope_models":
		output, ok := value["output"].(map[string]any)
		if !ok {
			return false
		}
		_, ok = output["models"].([]any)
		return ok
	}
	return false
}

func (s *Server) pingProvider(parent context.Context, p *store.ProviderRecord, key string, target *store.TargetRecord) providerTestResult {
	result := providerTestResult{Mode: "ping", State: "model_error", ModelName: target.ModelName}
	prov := adapter.Provider{ID: p.ID, Name: p.Name, Protocol: p.Protocol, BaseURL: p.BaseURL, Headers: p.Headers}
	maxTokens := 8
	tgt := adapter.Target{ModelName: target.ModelName, MaxTokensField: target.MaxTokensField, Core: adapter.CoreParams{MaxTokens: &maxTokens}}
	in := adapter.Input{Messages: []adapter.Message{{Role: "user", Content: json.RawMessage(`"ping"`)}}}
	body, err := adapter.BuildBody(prov, tgt, in)
	if err != nil {
		result.Error = fmt.Sprintf("build test body: %v", err)
		return result
	}
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	var ad adapter.Adapter
	if p.Protocol == "anthropic" {
		ad = &adapter.AnthropicAdapter{Client: s.HTTPClient, Clock: s.Clock}
	} else {
		ad = &adapter.OpenAIChatAdapter{Client: s.HTTPClient, Clock: s.Clock}
	}
	res := ad.Do(ctx, prov, key, body, nil)
	result.HTTPStatus, result.Error = res.HTTPStatus, res.Err
	result.Reachable = res.HTTPStatus != 0
	if res.Metrics.TotalMs != nil {
		result.TotalMs = *res.Metrics.TotalMs
	}
	if res.Status == "done" && (res.Output != "" || res.Reasoning != "") {
		result.OK, result.State = true, "connected"
	} else if res.HTTPStatus >= 400 {
		result.State = connectionHTTPState(res.HTTPStatus)
	} else if !result.Reachable {
		result.State = "connection_error"
	}
	if !result.OK && result.Error == "" {
		if ctx.Err() != nil {
			result.Error = ctx.Err().Error()
		} else {
			result.Error = "The model returned no output."
		}
	}
	return result
}
