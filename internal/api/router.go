package api

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"

	"doo/internal/adapter"
	"doo/internal/secrets"
	"doo/internal/store"
)

type Server struct {
	Store      *store.Store
	KeyStore   *secrets.KeyStore
	Version    string
	BuildTime  string
	GitCommit  string
	Security   SecurityConfig
	MaxTargets int
	Clock      adapter.Clock
	HTTPClient *http.Client

	cancelMu  sync.Mutex
	cancelMap map[int64]context.CancelFunc
}

func NewServer(s *store.Store, ks *secrets.KeyStore, version, buildTime, gitCommit string, sec SecurityConfig, maxTargets int) *Server {
	if maxTargets < 0 {
		maxTargets = 0
	}
	return &Server{
		Store:      s,
		KeyStore:   ks,
		Version:    version,
		BuildTime:  buildTime,
		GitCommit:  gitCommit,
		Security:   sec,
		MaxTargets: maxTargets,
		cancelMap:  make(map[int64]context.CancelFunc),
	}
}

func (s *Server) RegisterCancel(callID int64, cancel context.CancelFunc) {
	s.cancelMu.Lock()
	defer s.cancelMu.Unlock()
	s.cancelMap[callID] = cancel
}

func (s *Server) UnregisterCancel(callID int64) {
	s.cancelMu.Lock()
	defer s.cancelMu.Unlock()
	delete(s.cancelMap, callID)
}

func (s *Server) CancelCall(callID int64) bool {
	s.cancelMu.Lock()
	defer s.cancelMu.Unlock()
	if cancel, ok := s.cancelMap[callID]; ok {
		cancel()
		return true
	}
	return false
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("GET /api/health", s.handleHealth)

	// Providers
	mux.HandleFunc("GET /api/providers", s.handleListProviders)
	mux.HandleFunc("POST /api/providers", s.handleCreateProvider)
	mux.HandleFunc("GET /api/providers/{id}", s.handleGetProvider)
	mux.HandleFunc("PUT /api/providers/{id}", s.handleUpdateProvider)
	mux.HandleFunc("DELETE /api/providers/{id}", s.handleDeleteProvider)
	mux.HandleFunc("POST /api/providers/{id}/move", s.handleMoveProvider)
	mux.HandleFunc("POST /api/providers/{id}/test", s.handleTestProvider)
	mux.HandleFunc("GET /api/providers/{id}/models", s.handleFetchProviderModels)

	// Targets
	mux.HandleFunc("GET /api/targets", s.handleListTargets)
	mux.HandleFunc("POST /api/targets", s.handleCreateTarget)
	mux.HandleFunc("GET /api/targets/{id}", s.handleGetTarget)
	mux.HandleFunc("PUT /api/targets/{id}", s.handleUpdateTarget)
	mux.HandleFunc("DELETE /api/targets/{id}", s.handleDeleteTarget)
	mux.HandleFunc("POST /api/targets/{id}/duplicate", s.handleDuplicateTarget)

	// Preview & Runs
	mux.HandleFunc("POST /api/preview", s.handlePreview)
	mux.HandleFunc("POST /api/runs", s.handleCreateRun)
	mux.HandleFunc("POST /api/calls/{id}/cancel", s.handleCancelCall)

	// Runs history & calls
	mux.HandleFunc("GET /api/runs", s.handleListRuns)
	mux.HandleFunc("GET /api/runs/{id}", s.handleGetRun)
	mux.HandleFunc("DELETE /api/runs/{id}", s.handleDeleteRun)
	mux.HandleFunc("GET /api/calls/{id}/raw", s.handleGetCallRaw)
	mux.HandleFunc("GET /api/calls/{id}/export", s.handleExportCall)

	return SecurityMiddleware(s.Security, mux)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":          true,
		"version":     s.Version,
		"build_time":  s.BuildTime,
		"git_commit":  s.GitCommit,
		"max_targets": s.MaxTargets,
	})
}
