package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"doo/internal/adapter"
	"doo/internal/store"
)

type TargetPayload struct {
	ProviderID     int64              `json:"provider_id"`
	Label          string             `json:"label"`
	ModelName      string             `json:"model_name"`
	Core           adapter.CoreParams `json:"core"`
	Extra          map[string]any     `json:"extra"`
	MaxTokensField string             `json:"max_tokens_field"`
}

func (s *Server) handleListTargets(w http.ResponseWriter, r *http.Request) {
	list, err := s.Store.ListTargets()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if list == nil {
		list = make([]*store.TargetRecord, 0)
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreateTarget(w http.ResponseWriter, r *http.Request) {
	var payload TargetPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	payload.Label = strings.TrimSpace(payload.Label)
	payload.ModelName = strings.TrimSpace(payload.ModelName)

	if payload.ProviderID <= 0 || payload.Label == "" || payload.ModelName == "" {
		writeError(w, http.StatusBadRequest, "provider_id, label, and model_name are required")
		return
	}

	p, err := s.Store.GetProvider(payload.ProviderID)
	if err != nil || p == nil {
		writeError(w, http.StatusBadRequest, "invalid provider_id")
		return
	}

	if payload.MaxTokensField == "" {
		payload.MaxTokensField = adapter.DetermineMaxTokensField(payload.ModelName, "")
	}
	if payload.MaxTokensField != "max_tokens" && payload.MaxTokensField != "max_completion_tokens" {
		writeError(w, http.StatusBadRequest, "max_tokens_field must be max_tokens or max_completion_tokens")
		return
	}

	if payload.Core.Stream == nil {
		t := true
		payload.Core.Stream = &t
	}

	rec := &store.TargetRecord{
		ProviderID:     payload.ProviderID,
		Label:          payload.Label,
		ModelName:      payload.ModelName,
		Core:           payload.Core,
		Extra:          payload.Extra,
		MaxTokensField: payload.MaxTokensField,
	}

	id, err := s.Store.CreateTarget(rec)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("create target: %v", err))
		return
	}
	rec.ID = id

	writeJSON(w, http.StatusCreated, rec)
}

func (s *Server) handleGetTarget(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid target id")
		return
	}

	tgt, err := s.Store.GetTarget(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if tgt == nil {
		writeError(w, http.StatusNotFound, "target not found")
		return
	}

	writeJSON(w, http.StatusOK, tgt)
}

func (s *Server) handleUpdateTarget(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid target id")
		return
	}

	existing, err := s.Store.GetTarget(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "target not found")
		return
	}

	var payload TargetPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	payload.Label = strings.TrimSpace(payload.Label)
	payload.ModelName = strings.TrimSpace(payload.ModelName)

	if payload.ProviderID > 0 {
		p, err := s.Store.GetProvider(payload.ProviderID)
		if err != nil || p == nil {
			writeError(w, http.StatusBadRequest, "invalid provider_id")
			return
		}
		existing.ProviderID = payload.ProviderID
	}
	if payload.Label != "" {
		existing.Label = payload.Label
	}
	if payload.ModelName != "" {
		existing.ModelName = payload.ModelName
	}
	existing.Core = payload.Core
	existing.Extra = payload.Extra
	existing.CoreJSON = ""
	existing.ExtraJSON = ""

	if payload.MaxTokensField != "" {
		if payload.MaxTokensField != "max_tokens" && payload.MaxTokensField != "max_completion_tokens" {
			writeError(w, http.StatusBadRequest, "max_tokens_field must be max_tokens or max_completion_tokens")
			return
		}
		existing.MaxTokensField = payload.MaxTokensField
	} else if existing.MaxTokensField == "" {
		existing.MaxTokensField = adapter.DetermineMaxTokensField(existing.ModelName, "")
	}

	if err := s.Store.UpdateTarget(existing); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("update target: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, existing)
}

func (s *Server) handleDeleteTarget(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid target id")
		return
	}

	if err := s.Store.DeleteTarget(id); err != nil {
		writeError(w, http.StatusNotFound, "target not found or could not be deleted")
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleDuplicateTarget(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid target id")
		return
	}

	dup, err := s.Store.DuplicateTarget(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "target not found")
		return
	}

	writeJSON(w, http.StatusCreated, dup)
}
