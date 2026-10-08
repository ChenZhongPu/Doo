package store

import (
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"doo/internal/adapter"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Store struct {
	db *sql.DB
	mu sync.Mutex // serialize writes
}

type ProviderRecord struct {
	ID          int64             `json:"id"`
	Name        string            `json:"name"`
	Protocol    string            `json:"protocol"`
	BaseURL     string            `json:"base_url"`
	APIKeyRef   string            `json:"api_key_ref"`
	HeadersJSON string            `json:"headers_json"`
	Headers     map[string]string `json:"headers"`
	StreamUsage bool              `json:"stream_usage"`
	CreatedAt   int64             `json:"created_at"`
	UpdatedAt   int64             `json:"updated_at"`
}

type TargetRecord struct {
	ID             int64              `json:"id"`
	ProviderID     int64              `json:"provider_id"`
	Label          string             `json:"label"`
	ModelName      string             `json:"model_name"`
	CoreJSON       string             `json:"core_json"`
	Core           adapter.CoreParams `json:"core"`
	ExtraJSON      string             `json:"extra_json"`
	Extra          map[string]any     `json:"extra"`
	MaxTokensField string             `json:"max_tokens_field"`
	CreatedAt      int64              `json:"created_at"`
	UpdatedAt      int64              `json:"updated_at"`
}

type CallRecord struct {
	ID                 int64            `json:"id"`
	RunID              int64            `json:"run_id"`
	TargetID           *int64           `json:"target_id"`
	TargetSnapshotJSON string           `json:"target_snapshot_json"`
	Status             string           `json:"status"` // running, done, error, canceled
	RequestJSON        string           `json:"request_json"`
	RequestHeadersJSON string           `json:"request_headers_json"`
	ResponseRaw        []byte           `json:"-"`
	ResponseTruncated  bool             `json:"response_truncated"`
	OutputText         string           `json:"output_text"`
	ReasoningText      string           `json:"reasoning_text"`
	FinishReason       *string          `json:"finish_reason"`
	HTTPStatus         *int             `json:"http_status"`
	Error              *string          `json:"error"`
	UsageJSON          *string          `json:"usage_json"`
	PromptTokens       *int             `json:"prompt_tokens"`
	CompletionTokens   *int             `json:"completion_tokens"`
	ReasoningTokens    *int             `json:"reasoning_tokens"`
	TTFBms             *float64         `json:"ttfb_ms"`
	FirstTokenMs       *float64         `json:"first_token_ms"`
	FirstContentMs     *float64         `json:"first_content_ms"`
	TotalMs            *float64         `json:"total_ms"`
	ResponseModel      *string          `json:"response_model"`
	ProviderReqID      *string          `json:"provider_req_id"`
	StartedAt          int64            `json:"started_at"`
}

type RunSummaryCall struct {
	ID               int64    `json:"id"`
	TargetID         *int64   `json:"target_id"`
	Label            string   `json:"label"`
	Status           string   `json:"status"`
	TotalMs          *float64 `json:"total_ms"`
	PromptTokens     *int     `json:"prompt_tokens"`
	CompletionTokens *int     `json:"completion_tokens"`
	Error            *string  `json:"error"`
}

type RunSummary struct {
	ID        int64            `json:"id"`
	CreatedAt int64            `json:"created_at"`
	InputJSON string           `json:"input_json"`
	Note      string           `json:"note"`
	Calls     []RunSummaryCall `json:"calls"`
}

type RunDetail struct {
	ID        int64         `json:"id"`
	CreatedAt int64         `json:"created_at"`
	InputJSON string        `json:"input_json"`
	Note      string        `json:"note"`
	Calls     []*CallRecord `json:"calls"`
}

type CallRaw struct {
	RequestJSON    string `json:"request_json"`
	RequestHeaders string `json:"request_headers"`
	ResponseRaw    string `json:"response_raw"`
	Truncated      bool   `json:"truncated"`
}

func Open(dbPath string) (*Store, error) {
	if dbPath != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
			return nil, fmt.Errorf("create db dir: %w", err)
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// PRAGMA setup
	pragmas := []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA busy_timeout=5000;",
		"PRAGMA foreign_keys=ON;",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("exec %s: %w", p, err)
		}
	}

	db.SetMaxOpenConns(1)

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	// Startup cleanup: recover running calls to interrupted
	if _, err := db.Exec("UPDATE calls SET status='error', error='interrupted' WHERE status='running'"); err != nil {
		db.Close()
		return nil, fmt.Errorf("recover running calls: %w", err)
	}

	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	var version int
	if err := s.db.QueryRow("PRAGMA user_version;").Scan(&version); err != nil {
		return err
	}

	if version < 1 {
		sqlBytes, err := migrationsFS.ReadFile("migrations/0001_init.sql")
		if err != nil {
			return fmt.Errorf("read 0001_init.sql: %w", err)
		}

		if _, err := s.db.Exec(string(sqlBytes)); err != nil {
			return fmt.Errorf("exec 0001_init.sql: %w", err)
		}

		if _, err := s.db.Exec("PRAGMA user_version = 1;"); err != nil {
			return fmt.Errorf("set user_version: %w", err)
		}
	}

	return nil
}

// Providers CRUD

func (s *Store) CreateProvider(p *ProviderRecord) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli()
	p.CreatedAt = now
	p.UpdatedAt = now

	headersJSON := p.HeadersJSON
	if headersJSON == "" {
		if p.Headers != nil {
			b, _ := json.Marshal(p.Headers)
			headersJSON = string(b)
		} else {
			headersJSON = "{}"
		}
	}
	streamUsageInt := 0
	if p.StreamUsage {
		streamUsageInt = 1
	}

	res, err := s.db.Exec(`
		INSERT INTO providers(name, protocol, base_url, api_key_ref, headers_json, stream_usage, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?)
	`, p.Name, p.Protocol, p.BaseURL, p.APIKeyRef, headersJSON, streamUsageInt, p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	p.ID = id
	return id, nil
}

func (s *Store) GetProvider(id int64) (*ProviderRecord, error) {
	row := s.db.QueryRow(`
		SELECT id, name, protocol, base_url, api_key_ref, headers_json, stream_usage, created_at, updated_at
		FROM providers WHERE id = ?
	`, id)

	var p ProviderRecord
	var streamUsageInt int
	if err := row.Scan(&p.ID, &p.Name, &p.Protocol, &p.BaseURL, &p.APIKeyRef, &p.HeadersJSON, &streamUsageInt, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	p.StreamUsage = streamUsageInt == 1
	_ = json.Unmarshal([]byte(p.HeadersJSON), &p.Headers)
	return &p, nil
}

func (s *Store) ListProviders() ([]*ProviderRecord, error) {
	rows, err := s.db.Query(`
		SELECT id, name, protocol, base_url, api_key_ref, headers_json, stream_usage, created_at, updated_at
		FROM providers ORDER BY id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]*ProviderRecord, 0)
	for rows.Next() {
		var p ProviderRecord
		var streamUsageInt int
		if err := rows.Scan(&p.ID, &p.Name, &p.Protocol, &p.BaseURL, &p.APIKeyRef, &p.HeadersJSON, &streamUsageInt, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		p.StreamUsage = streamUsageInt == 1
		_ = json.Unmarshal([]byte(p.HeadersJSON), &p.Headers)
		list = append(list, &p)
	}
	return list, rows.Err()
}

func (s *Store) UpdateProvider(p *ProviderRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	p.UpdatedAt = time.Now().UnixMilli()
	headersJSON := p.HeadersJSON
	if headersJSON == "" {
		if p.Headers != nil {
			b, _ := json.Marshal(p.Headers)
			headersJSON = string(b)
		} else {
			headersJSON = "{}"
		}
	}
	streamUsageInt := 0
	if p.StreamUsage {
		streamUsageInt = 1
	}

	res, err := s.db.Exec(`
		UPDATE providers
		SET name = ?, protocol = ?, base_url = ?, api_key_ref = ?, headers_json = ?, stream_usage = ?, updated_at = ?
		WHERE id = ?
	`, p.Name, p.Protocol, p.BaseURL, p.APIKeyRef, headersJSON, streamUsageInt, p.UpdatedAt, p.ID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) DeleteProvider(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	res, err := s.db.Exec("DELETE FROM providers WHERE id = ?", id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// Targets CRUD

func (s *Store) CreateTarget(t *TargetRecord) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli()
	t.CreatedAt = now
	t.UpdatedAt = now

	coreJSON := t.CoreJSON
	if coreJSON == "" {
		b, _ := json.Marshal(t.Core)
		coreJSON = string(b)
	}
	extraJSON := t.ExtraJSON
	if extraJSON == "" {
		if t.Extra != nil {
			b, _ := json.Marshal(t.Extra)
			extraJSON = string(b)
		} else {
			extraJSON = "{}"
		}
	}
	if t.MaxTokensField == "" {
		t.MaxTokensField = "max_tokens"
	}

	res, err := s.db.Exec(`
		INSERT INTO targets(provider_id, label, model_name, core_json, extra_json, max_tokens_field, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?)
	`, t.ProviderID, t.Label, t.ModelName, coreJSON, extraJSON, t.MaxTokensField, t.CreatedAt, t.UpdatedAt)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	t.ID = id
	return id, nil
}

func (s *Store) GetTarget(id int64) (*TargetRecord, error) {
	row := s.db.QueryRow(`
		SELECT id, provider_id, label, model_name, core_json, extra_json, max_tokens_field, created_at, updated_at
		FROM targets WHERE id = ?
	`, id)

	var t TargetRecord
	if err := row.Scan(&t.ID, &t.ProviderID, &t.Label, &t.ModelName, &t.CoreJSON, &t.ExtraJSON, &t.MaxTokensField, &t.CreatedAt, &t.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	_ = json.Unmarshal([]byte(t.CoreJSON), &t.Core)
	_ = json.Unmarshal([]byte(t.ExtraJSON), &t.Extra)
	return &t, nil
}

func (s *Store) ListTargets() ([]*TargetRecord, error) {
	rows, err := s.db.Query(`
		SELECT id, provider_id, label, model_name, core_json, extra_json, max_tokens_field, created_at, updated_at
		FROM targets ORDER BY id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]*TargetRecord, 0)
	for rows.Next() {
		var t TargetRecord
		if err := rows.Scan(&t.ID, &t.ProviderID, &t.Label, &t.ModelName, &t.CoreJSON, &t.ExtraJSON, &t.MaxTokensField, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(t.CoreJSON), &t.Core)
		_ = json.Unmarshal([]byte(t.ExtraJSON), &t.Extra)
		list = append(list, &t)
	}
	return list, rows.Err()
}

func (s *Store) UpdateTarget(t *TargetRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	t.UpdatedAt = time.Now().UnixMilli()
	coreJSON := t.CoreJSON
	if coreJSON == "" {
		b, _ := json.Marshal(t.Core)
		coreJSON = string(b)
	}
	extraJSON := t.ExtraJSON
	if extraJSON == "" {
		if t.Extra != nil {
			b, _ := json.Marshal(t.Extra)
			extraJSON = string(b)
		} else {
			extraJSON = "{}"
		}
	}
	if t.MaxTokensField == "" {
		t.MaxTokensField = "max_tokens"
	}

	res, err := s.db.Exec(`
		UPDATE targets
		SET provider_id = ?, label = ?, model_name = ?, core_json = ?, extra_json = ?, max_tokens_field = ?, updated_at = ?
		WHERE id = ?
	`, t.ProviderID, t.Label, t.ModelName, coreJSON, extraJSON, t.MaxTokensField, t.UpdatedAt, t.ID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) DeleteTarget(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	res, err := s.db.Exec("DELETE FROM targets WHERE id = ?", id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) DuplicateTarget(id int64) (*TargetRecord, error) {
	orig, err := s.GetTarget(id)
	if err != nil {
		return nil, err
	}
	if orig == nil {
		return nil, sql.ErrNoRows
	}

	copyTarget := &TargetRecord{
		ProviderID:     orig.ProviderID,
		Label:          orig.Label + " (copy)",
		ModelName:      orig.ModelName,
		CoreJSON:       orig.CoreJSON,
		Core:           orig.Core,
		ExtraJSON:      orig.ExtraJSON,
		Extra:          orig.Extra,
		MaxTokensField: orig.MaxTokensField,
	}

	newID, err := s.CreateTarget(copyTarget)
	if err != nil {
		return nil, err
	}
	copyTarget.ID = newID
	return copyTarget, nil
}

// Runs & Calls

func (s *Store) CreateRun(inputJSON string, note string, calls []*CallRecord) (int64, []int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli()
	res, err := s.db.Exec("INSERT INTO runs(created_at, input_json, note) VALUES(?, ?, ?)", now, inputJSON, note)
	if err != nil {
		return 0, nil, fmt.Errorf("insert run: %w", err)
	}
	runID, err := res.LastInsertId()
	if err != nil {
		return 0, nil, err
	}

	callIDs := make([]int64, len(calls))
	for i, c := range calls {
		c.RunID = runID
		c.StartedAt = now
		c.Status = "running"
		if c.RequestHeadersJSON == "" {
			c.RequestHeadersJSON = "{}"
		}

		cRes, err := s.db.Exec(`
			INSERT INTO calls(
				run_id, target_id, target_snapshot_json, status, request_json, request_headers_json,
				response_truncated, output_text, reasoning_text, started_at
			) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, c.RunID, c.TargetID, c.TargetSnapshotJSON, c.Status, c.RequestJSON, c.RequestHeadersJSON,
			0, "", "", c.StartedAt)
		if err != nil {
			return 0, nil, fmt.Errorf("insert call: %w", err)
		}
		cID, err := cRes.LastInsertId()
		if err != nil {
			return 0, nil, err
		}
		c.ID = cID
		callIDs[i] = cID
	}

	return runID, callIDs, nil
}

func (s *Store) UpdateCallFinished(callID int64, res *adapter.Result) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	truncInt := 0
	if res.RespTruncated {
		truncInt = 1
	}

	var finishReason *string
	if res.FinishReason != "" {
		finishReason = &res.FinishReason
	}
	var errStr *string
	if res.Err != "" {
		errStr = &res.Err
	}
	var usageStr *string
	if len(res.UsageJSON) > 0 {
		str := string(res.UsageJSON)
		usageStr = &str
	}
	var respModel *string
	if res.ResponseModel != "" {
		respModel = &res.ResponseModel
	}
	var reqID *string
	if res.ProviderReqID != "" {
		reqID = &res.ProviderReqID
	}

	_, err := s.db.Exec(`
		UPDATE calls
		SET status = ?, response_raw = ?, response_truncated = ?, output_text = ?, reasoning_text = ?,
		    finish_reason = ?, http_status = ?, error = ?, usage_json = ?, prompt_tokens = ?,
		    completion_tokens = ?, reasoning_tokens = ?, ttfb_ms = ?, first_token_ms = ?,
		    first_content_ms = ?, total_ms = ?, response_model = ?, provider_req_id = ?
		WHERE id = ?
	`, res.Status, res.RespRaw, truncInt, res.Output, res.Reasoning,
		finishReason, res.HTTPStatus, errStr, usageStr, res.PromptTokens,
		res.CompletionTokens, res.ReasoningTokens, res.Metrics.TTFBms, res.Metrics.FirstTokenMs,
		res.Metrics.FirstContentMs, res.Metrics.TotalMs, respModel, reqID, callID)

	return err
}

func (s *Store) GetCall(callID int64) (*CallRecord, error) {
	row := s.db.QueryRow(`
		SELECT id, run_id, target_id, target_snapshot_json, status, request_json, request_headers_json,
		       response_truncated, output_text, reasoning_text, finish_reason, http_status, error,
		       usage_json, prompt_tokens, completion_tokens, reasoning_tokens, ttfb_ms, first_token_ms,
		       first_content_ms, total_ms, response_model, provider_req_id, started_at
		FROM calls WHERE id = ?
	`, callID)

	var c CallRecord
	var truncInt int
	if err := row.Scan(
		&c.ID, &c.RunID, &c.TargetID, &c.TargetSnapshotJSON, &c.Status, &c.RequestJSON, &c.RequestHeadersJSON,
		&truncInt, &c.OutputText, &c.ReasoningText, &c.FinishReason, &c.HTTPStatus, &c.Error,
		&c.UsageJSON, &c.PromptTokens, &c.CompletionTokens, &c.ReasoningTokens, &c.TTFBms, &c.FirstTokenMs,
		&c.FirstContentMs, &c.TotalMs, &c.ResponseModel, &c.ProviderReqID, &c.StartedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	c.ResponseTruncated = truncInt == 1
	return &c, nil
}

func (s *Store) GetCallRaw(callID int64) (*CallRaw, error) {
	row := s.db.QueryRow(`
		SELECT request_json, request_headers_json, response_raw, response_truncated
		FROM calls WHERE id = ?
	`, callID)

	var reqJSON, reqHeaders string
	var rawBytes []byte
	var truncInt int
	if err := row.Scan(&reqJSON, &reqHeaders, &rawBytes, &truncInt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &CallRaw{
		RequestJSON:    reqJSON,
		RequestHeaders: reqHeaders,
		ResponseRaw:    string(rawBytes),
		Truncated:      truncInt == 1,
	}, nil
}

func (s *Store) GetRun(runID int64) (*RunDetail, error) {
	row := s.db.QueryRow("SELECT id, created_at, input_json, note FROM runs WHERE id = ?", runID)
	var r RunDetail
	r.Calls = make([]*CallRecord, 0)
	if err := row.Scan(&r.ID, &r.CreatedAt, &r.InputJSON, &r.Note); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	rows, err := s.db.Query(`
		SELECT id, run_id, target_id, target_snapshot_json, status, request_json, request_headers_json,
		       response_truncated, output_text, reasoning_text, finish_reason, http_status, error,
		       usage_json, prompt_tokens, completion_tokens, reasoning_tokens, ttfb_ms, first_token_ms,
		       first_content_ms, total_ms, response_model, provider_req_id, started_at
		FROM calls WHERE run_id = ? ORDER BY id ASC
	`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var c CallRecord
		var truncInt int
		if err := rows.Scan(
			&c.ID, &c.RunID, &c.TargetID, &c.TargetSnapshotJSON, &c.Status, &c.RequestJSON, &c.RequestHeadersJSON,
			&truncInt, &c.OutputText, &c.ReasoningText, &c.FinishReason, &c.HTTPStatus, &c.Error,
			&c.UsageJSON, &c.PromptTokens, &c.CompletionTokens, &c.ReasoningTokens, &c.TTFBms, &c.FirstTokenMs,
			&c.FirstContentMs, &c.TotalMs, &c.ResponseModel, &c.ProviderReqID, &c.StartedAt,
		); err != nil {
			return nil, err
		}
		c.ResponseTruncated = truncInt == 1
		r.Calls = append(r.Calls, &c)
	}

	return &r, rows.Err()
}

func (s *Store) ListRuns(limit int, beforeID int64) ([]*RunSummary, error) {
	if limit <= 0 {
		limit = 50
	}

	var query string
	var args []any
	if beforeID > 0 {
		query = "SELECT id, created_at, input_json, note FROM runs WHERE id < ? ORDER BY id DESC LIMIT ?"
		args = []any{beforeID, limit}
	} else {
		query = "SELECT id, created_at, input_json, note FROM runs ORDER BY id DESC LIMIT ?"
		args = []any{limit}
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	runs := make([]*RunSummary, 0)
	runMap := make(map[int64]*RunSummary)
	var runIDs []int64

	for rows.Next() {
		var r RunSummary
		if err := rows.Scan(&r.ID, &r.CreatedAt, &r.InputJSON, &r.Note); err != nil {
			return nil, err
		}
		r.Calls = []RunSummaryCall{}
		runs = append(runs, &r)
		runMap[r.ID] = &r
		runIDs = append(runIDs, r.ID)
	}

	if len(runIDs) == 0 {
		return runs, nil
	}

	// Fetch calls for these runs
	callRows, err := s.db.Query(`
		SELECT c.id, c.run_id, c.target_id, c.target_snapshot_json, c.status, c.total_ms,
		       c.prompt_tokens, c.completion_tokens, c.error
		FROM calls c
		WHERE c.run_id IN (` + makePlaceholders(len(runIDs)) + `)
		ORDER BY c.id ASC
	`, toAnySlice(runIDs)...)
	if err != nil {
		return nil, err
	}
	defer callRows.Close()

	for callRows.Next() {
		var callID, runID int64
		var targetID *int64
		var snapshotJSON, status string
		var totalMs *float64
		var promptTokens, completionTokens *int
		var errStr *string

		if err := callRows.Scan(&callID, &runID, &targetID, &snapshotJSON, &status, &totalMs,
			&promptTokens, &completionTokens, &errStr); err != nil {
			return nil, err
		}

		label := ""
		var snap struct {
			Target struct {
				Label string `json:"label"`
			} `json:"target"`
		}
		if json.Unmarshal([]byte(snapshotJSON), &snap) == nil {
			label = snap.Target.Label
		}

		if r, ok := runMap[runID]; ok {
			r.Calls = append(r.Calls, RunSummaryCall{
				ID:               callID,
				TargetID:         targetID,
				Label:            label,
				Status:           status,
				TotalMs:          totalMs,
				PromptTokens:     promptTokens,
				CompletionTokens: completionTokens,
				Error:            errStr,
			})
		}
	}

	return runs, nil
}

func (s *Store) DeleteRun(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	res, err := s.db.Exec("DELETE FROM runs WHERE id = ?", id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func makePlaceholders(n int) string {
	if n <= 0 {
		return ""
	}
	b := make([]byte, n*2-1)
	for i := 0; i < n; i++ {
		b[i*2] = '?'
		if i < n-1 {
			b[i*2+1] = ','
		}
	}
	return string(b)
}

func toAnySlice(ids []int64) []any {
	out := make([]any, len(ids))
	for i, id := range ids {
		out[i] = id
	}
	return out
}
