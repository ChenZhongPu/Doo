package store

import (
	"path/filepath"
	"testing"

	"doo/internal/adapter"
)

func TestStore_MigrationAndCRUD(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer s.Close()

	// 1. Create Provider
	p := &ProviderRecord{
		Name:        "OpenAI",
		Protocol:    "openai_chat",
		BaseURL:     "https://api.openai.com/v1",
		APIKeyRef:   "env:OPENAI_API_KEY",
		Headers:     map[string]string{"User-Agent": "test"},
		StreamUsage: true,
	}
	pID, err := s.CreateProvider(p)
	if err != nil {
		t.Fatalf("create provider failed: %v", err)
	}
	if pID == 0 {
		t.Fatalf("expected non-zero provider id")
	}

	gotP, err := s.GetProvider(pID)
	if err != nil || gotP == nil {
		t.Fatalf("get provider failed: %v", err)
	}
	if gotP.Name != "OpenAI" || gotP.StreamUsage != true {
		t.Errorf("mismatch provider fields: %+v", gotP)
	}

	// 2. Create Target
	temp := 0.7
	tgt := &TargetRecord{
		ProviderID: pID,
		Label:      "gpt-4o",
		ModelName:  "gpt-4o",
		Core: adapter.CoreParams{
			Temperature: &temp,
		},
		MaxTokensField: "max_tokens",
	}
	tID, err := s.CreateTarget(tgt)
	if err != nil {
		t.Fatalf("create target failed: %v", err)
	}

	gotTgt, err := s.GetTarget(tID)
	if err != nil || gotTgt == nil {
		t.Fatalf("get target failed: %v", err)
	}
	if gotTgt.Core.Temperature == nil || *gotTgt.Core.Temperature != 0.7 {
		t.Errorf("mismatch target core params: %+v", gotTgt.Core)
	}

	// 3. Duplicate Target
	dupTgt, err := s.DuplicateTarget(tID)
	if err != nil {
		t.Fatalf("duplicate target failed: %v", err)
	}
	if dupTgt.Label != "gpt-4o (copy)" {
		t.Errorf("expected label gpt-4o (copy), got %s", dupTgt.Label)
	}

	// 4. Create Run & Call
	calls := []*CallRecord{
		{
			TargetID:           &tID,
			TargetSnapshotJSON: `{"target":{"label":"gpt-4o"}}`,
			RequestJSON:        `{"model":"gpt-4o"}`,
			RequestHeadersJSON: `{"Content-Type":"application/json"}`,
		},
	}
	runID, callIDs, err := s.CreateRun(`{"messages":[{"role":"user","content":"hi"}]}`, "test note", calls)
	if err != nil {
		t.Fatalf("create run failed: %v", err)
	}
	if len(callIDs) != 1 {
		t.Fatalf("expected 1 call id, got %d", len(callIDs))
	}

	// Update call finished
	cID := callIDs[0]
	ttfb := 100.0
	total := 500.0
	promptTokens := 10
	compTokens := 20
	finishReason := "stop"
	callRes := &adapter.Result{
		Status:           "done",
		HTTPStatus:       200,
		Output:           "Hello world",
		FinishReason:     finishReason,
		RespRaw:          []byte("raw sse response"),
		PromptTokens:     &promptTokens,
		CompletionTokens: &compTokens,
		Metrics: adapter.Metrics{
			TTFBms:  &ttfb,
			TotalMs: &total,
		},
	}
	if err := s.UpdateCallFinished(cID, callRes); err != nil {
		t.Fatalf("update call finished failed: %v", err)
	}

	// Check RunDetail
	runDetail, err := s.GetRun(runID)
	if err != nil || runDetail == nil {
		t.Fatalf("get run failed: %v", err)
	}
	if len(runDetail.Calls) != 1 {
		t.Fatalf("expected 1 call in run, got %d", len(runDetail.Calls))
	}
	if runDetail.Calls[0].OutputText != "Hello world" {
		t.Errorf("expected output Hello world, got %s", runDetail.Calls[0].OutputText)
	}

	// Check CallRaw
	raw, err := s.GetCallRaw(cID)
	if err != nil || raw == nil {
		t.Fatalf("get call raw failed: %v", err)
	}
	if raw.ResponseRaw != "raw sse response" {
		t.Errorf("expected raw sse response, got %s", raw.ResponseRaw)
	}

	// Check ListRuns
	summaries, err := s.ListRuns(10, 0)
	if err != nil {
		t.Fatalf("list runs failed: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}
	if len(summaries[0].Calls) != 1 || summaries[0].Calls[0].Label != "gpt-4o" {
		t.Errorf("summary calls mismatch: %+v", summaries[0].Calls)
	}

	// Check Cascade Delete
	if err := s.DeleteProvider(pID); err != nil {
		t.Fatalf("delete provider failed: %v", err)
	}
	remainingTargets, err := s.ListTargets()
	if err != nil {
		t.Fatalf("list targets failed: %v", err)
	}
	if len(remainingTargets) != 0 {
		t.Errorf("targets should be cascade deleted, got %d", len(remainingTargets))
	}
}
