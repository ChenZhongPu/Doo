package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"doo/internal/adapter"
	"doo/internal/secrets"
	"doo/internal/store"
)

type PreviewRequest struct {
	TargetID int64         `json:"target_id"`
	Input    adapter.Input `json:"input"`
}

type PreviewResponse struct {
	Body    string            `json:"body"`
	Headers map[string]string `json:"headers"`
}

type CreateRunRequest struct {
	Input     adapter.Input `json:"input"`
	TargetIDs []int64       `json:"target_ids"`
	TimeoutS  int           `json:"timeout_s"`
	Note      string        `json:"note"`
}

func (s *Server) handlePreview(w http.ResponseWriter, r *http.Request) {
	var req PreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	tgtRec, err := s.Store.GetTarget(req.TargetID)
	if err != nil || tgtRec == nil {
		writeError(w, http.StatusBadRequest, "target not found")
		return
	}

	pRec, err := s.Store.GetProvider(tgtRec.ProviderID)
	if err != nil || pRec == nil {
		writeError(w, http.StatusBadRequest, "provider not found")
		return
	}

	p := toAdapterProvider(pRec)
	t := toAdapterTarget(tgtRec)

	body, err := adapter.BuildBody(p, t, req.Input)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("build body: %v", err))
		return
	}

	headers := buildMaskedHeaders(p)

	writeJSON(w, http.StatusOK, PreviewResponse{
		Body:    string(body),
		Headers: headers,
	})
}

func (s *Server) handleCreateRun(w http.ResponseWriter, r *http.Request) {
	var req CreateRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.TargetIDs) == 0 {
		writeError(w, http.StatusBadRequest, "target_ids cannot be empty")
		return
	}
	if s.MaxTargets > 0 && len(req.TargetIDs) > s.MaxTargets {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("target_ids cannot exceed %d", s.MaxTargets))
		return
	}

	type preparedTarget struct {
		target   adapter.Target
		provider adapter.Provider
		apiKey   string
		body     []byte
		headers  map[string]string
		rawSnap  string
	}

	prepared := make([]preparedTarget, len(req.TargetIDs))

	// Step 1: Pre-validation of all targets. If ANY fails, return 400 immediately!
	for i, tgtID := range req.TargetIDs {
		tgtRec, err := s.Store.GetTarget(tgtID)
		if err != nil || tgtRec == nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("target id %d not found", tgtID))
			return
		}

		pRec, err := s.Store.GetProvider(tgtRec.ProviderID)
		if err != nil || pRec == nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("target %q: provider id %d not found", tgtRec.Label, tgtRec.ProviderID))
			return
		}

		p := toAdapterProvider(pRec)
		t := toAdapterTarget(tgtRec)

		apiKey, err := s.KeyStore.Resolve(p.APIKeyRef)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("target %q: %v", tgtRec.Label, err))
			return
		}

		body, err := adapter.BuildBody(p, t, req.Input)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("target %q: %v", tgtRec.Label, err))
			return
		}

		headers := buildMaskedHeaders(p)

		// Create snapshot
		snap := struct {
			Provider adapter.Provider `json:"provider"`
			Target   adapter.Target   `json:"target"`
		}{
			Provider: adapter.Provider{
				ID:          p.ID,
				Name:        p.Name,
				Protocol:    p.Protocol,
				BaseURL:     p.BaseURL,
				APIKeyRef:   secrets.MaskRef(p.APIKeyRef),
				Headers:     headers,
				StreamUsage: p.StreamUsage,
			},
			Target: t,
		}
		snapBytes, _ := json.Marshal(snap)

		prepared[i] = preparedTarget{
			target:   t,
			provider: p,
			apiKey:   apiKey,
			body:     body,
			headers:  headers,
			rawSnap:  string(snapBytes),
		}
	}

	// Step 2: Write run and calls to DB with status=running
	inputBytes, _ := json.Marshal(req.Input)
	calls := make([]*store.CallRecord, len(prepared))
	for i, prep := range prepared {
		tgtID := prep.target.ID
		hBytes, _ := json.Marshal(prep.headers)
		calls[i] = &store.CallRecord{
			TargetID:           &tgtID,
			TargetSnapshotJSON: prep.rawSnap,
			RequestJSON:        string(prep.body),
			RequestHeadersJSON: string(hBytes),
		}
	}

	runID, callIDs, err := s.Store.CreateRun(string(inputBytes), req.Note, calls)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("create run: %v", err))
		return
	}

	// Step 3: Setup SSE response
	sse, err := NewSSEWriter(w)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	runEvent := SSERunEvent{
		Type:  "run",
		RunID: runID,
		Calls: make([]SSERunCallInfo, len(prepared)),
	}
	for i, prep := range prepared {
		runEvent.Calls[i] = SSERunCallInfo{
			CallID:   callIDs[i],
			TargetID: prep.target.ID,
			Label:    prep.target.Label,
		}
	}
	_ = sse.Send(runEvent)

	// Step 4: Run calls concurrently
	timeoutS := req.TimeoutS
	if timeoutS <= 0 {
		timeoutS = 300
	}

	runCtx, cancelAll := context.WithTimeout(r.Context(), time.Duration(timeoutS)*time.Second)
	defer cancelAll()

	var wg sync.WaitGroup

	for i, prep := range prepared {
		wg.Add(1)
		callID := callIDs[i]
		prepCopy := prep

		go func() {
			defer wg.Done()

			callCtx, cancelCall := context.WithCancel(runCtx)
			s.RegisterCancel(callID, cancelCall)
			defer s.UnregisterCancel(callID)

			var ad adapter.Adapter
			if prepCopy.provider.Protocol == "anthropic" {
				ad = &adapter.AnthropicAdapter{Client: s.HTTPClient, Clock: s.Clock}
			} else {
				ad = &adapter.OpenAIChatAdapter{Client: s.HTTPClient, Clock: s.Clock}
			}

			res := ad.Do(callCtx, prepCopy.provider, prepCopy.apiKey, prepCopy.body, func(delta adapter.Event) {
				_ = sse.Send(SSEDeltaEvent{
					Type:   "delta",
					CallID: callID,
					Kind:   delta.Kind,
					Text:   delta.Text,
				})
			})

			// Save result to DB
			_ = s.Store.UpdateCallFinished(callID, res)

			// Send done or error event
			if res.Status == "done" || res.Status == "canceled" {
				_ = sse.Send(SSEDoneEvent{
					Type:          "done",
					CallID:        callID,
					Status:        res.Status,
					FinishReason:  res.FinishReason,
					Usage:         res.UsageJSON,
					Metrics:       res.Metrics,
					ResponseModel: res.ResponseModel,
					HTTPStatus:    res.HTTPStatus,
				})
			} else {
				_ = sse.Send(SSEErrorEvent{
					Type:       "error",
					CallID:     callID,
					HTTPStatus: res.HTTPStatus,
					Message:    res.Err,
					Metrics:    res.Metrics,
					Output:     res.Output,
				})
			}
		}()
	}

	wg.Wait()
	_ = sse.Send(SSEEndEvent{Type: "end"})
}

func (s *Server) handleCancelCall(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid call id")
		return
	}

	canceled := s.CancelCall(id)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": canceled})
}

func (s *Server) handleListRuns(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	var beforeID int64
	if b := r.URL.Query().Get("before"); b != "" {
		if parsed, err := strconv.ParseInt(b, 10, 64); err == nil {
			beforeID = parsed
		}
	}

	runs, err := s.Store.ListRuns(limit, beforeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if runs == nil {
		runs = make([]*store.RunSummary, 0)
	}
	writeJSON(w, http.StatusOK, runs)
}

func (s *Server) handleGetRun(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid run id")
		return
	}

	run, err := s.Store.GetRun(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if run == nil {
		writeError(w, http.StatusNotFound, "run not found")
		return
	}

	writeJSON(w, http.StatusOK, run)
}

func (s *Server) handleDeleteRun(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid run id")
		return
	}

	if err := s.Store.DeleteRun(id); err != nil {
		writeError(w, http.StatusNotFound, "run not found or could not be deleted")
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleGetCallRaw(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid call id")
		return
	}

	raw, err := s.Store.GetCallRaw(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if raw == nil {
		writeError(w, http.StatusNotFound, "call not found")
		return
	}

	writeJSON(w, http.StatusOK, raw)
}

func toAdapterProvider(p *store.ProviderRecord) adapter.Provider {
	return adapter.Provider{
		ID:          p.ID,
		Name:        p.Name,
		Protocol:    p.Protocol,
		BaseURL:     p.BaseURL,
		APIKeyRef:   p.APIKeyRef,
		Headers:     p.Headers,
		StreamUsage: p.StreamUsage,
	}
}

func toAdapterTarget(t *store.TargetRecord) adapter.Target {
	return adapter.Target{
		ID:             t.ID,
		ProviderID:     t.ProviderID,
		Label:          t.Label,
		ModelName:      t.ModelName,
		Core:           t.Core,
		Extra:          t.Extra,
		MaxTokensField: t.MaxTokensField,
	}
}

func buildMaskedHeaders(p adapter.Provider) map[string]string {
	h := make(map[string]string)
	h["Content-Type"] = "application/json"
	h["Accept"] = "text/event-stream"
	if p.Protocol == "openai_chat" {
		h["Authorization"] = "Bearer ***"
	} else if p.Protocol == "anthropic" {
		h["x-api-key"] = "***"
		h["anthropic-version"] = "2023-06-01"
	}
	for k, v := range secrets.MaskHeaders(p.Headers) {
		h[k] = v
	}
	return h
}
