package store

import (
	"path/filepath"
	"testing"
)

func seedHistory(t *testing.T, s *Store, count int) {
	t.Helper()
	for i := 1; i <= count; i++ {
		id, _, err := s.CreateRun(`{"messages":[{"role":"user","content":"test"}]}`, "", []*CallRecord{{TargetSnapshotJSON: `{"target":{"label":"Baseline"}}`, RequestJSON: "{}", RequestHeadersJSON: "{}"}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.db.Exec("UPDATE runs SET created_at = ? WHERE id = ?", i*1000, id); err != nil {
			t.Fatal(err)
		}
	}
}

func TestHistoryPagination(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	empty, err := s.ListRunsPage(999)
	if err != nil || empty.Page != 1 || empty.Total != 0 || len(empty.Runs) != 0 {
		t.Fatalf("empty: %+v %v", empty, err)
	}
	seedHistory(t, s, 205)
	for _, tc := range []struct {
		requested, page, first, last int64
		count                        int
	}{{1, 1, 205, 106, 100}, {2, 2, 105, 6, 100}, {3, 3, 5, 1, 5}, {999, 3, 5, 1, 5}} {
		p, err := s.ListRunsPage(tc.requested)
		if err != nil {
			t.Fatal(err)
		}
		if p.Total != 205 || p.PageSize != 100 || p.Page != tc.page || len(p.Runs) != tc.count || p.Runs[0].ID != tc.first || p.Runs[len(p.Runs)-1].ID != tc.last {
			t.Fatalf("page %d: %+v", tc.requested, p)
		}
		if len(p.Runs[0].Calls) != 1 || p.Runs[0].Calls[0].Label != "Baseline" {
			t.Fatal("missing call summary")
		}
	}
	if _, err := s.ListRunsPage(0); err == nil {
		t.Fatal("accepted page zero")
	}
	_, err = s.DeleteHistory(HistoryFilter{IDs: []int64{1, 2, 3, 4, 5}}, false)
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.ListRunsPage(3)
	if err != nil || p.Page != 2 || p.Total != 200 || len(p.Runs) != 100 {
		t.Fatalf("clamp after deletion: %+v %v", p, err)
	}
}

func TestHistoryDeletion(t *testing.T) {
	before, keep, negative := int64(3000), int64(2), int64(-1)
	for _, tc := range []struct {
		name    string
		filter  HistoryFilter
		removed int64
	}{
		{"all", HistoryFilter{All: true}, 5}, {"before exclusive", HistoryFilter{BeforeMS: &before}, 2},
		{"IDs", HistoryFilter{IDs: []int64{2, 4, 4, 999}}, 2}, {"keep last", HistoryFilter{KeepLast: &keep}, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, err := Open(filepath.Join(t.TempDir(), "history.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			seedHistory(t, s, 5)
			providerID, err := s.CreateProvider(&ProviderRecord{Name: "Kept", Protocol: "openai_chat", BaseURL: "http://localhost/v1", APIKeyRef: "env:TEST_KEY"})
			if err != nil {
				t.Fatal(err)
			}
			targetID, err := s.CreateTarget(&TargetRecord{ProviderID: providerID, Label: "Kept", ModelName: "model", MaxTokensField: "max_tokens"})
			if err != nil {
				t.Fatal(err)
			}
			count, err := s.DeleteHistory(tc.filter, true)
			if err != nil || count != tc.removed {
				t.Fatalf("preview %d %v", count, err)
			}
			p, _ := s.ListRunsPage(1)
			if p.Total != 5 {
				t.Fatal("dry-run changed history")
			}
			count, err = s.DeleteHistory(tc.filter, false)
			if err != nil || count != tc.removed {
				t.Fatalf("delete %d %v", count, err)
			}
			p, _ = s.ListRunsPage(1)
			if p.Total != 5-tc.removed {
				t.Fatal("wrong remaining count")
			}
			for _, run := range p.Runs {
				if tc.name == "before exclusive" && run.CreatedAt < before {
					t.Fatal("old record remains")
				}
				if tc.name == "keep last" && run.ID < 4 {
					t.Fatal("wrong latest records retained")
				}
			}
			var calls int64
			if err := s.db.QueryRow("SELECT COUNT(*) FROM calls").Scan(&calls); err != nil || calls != p.Total {
				t.Fatalf("call cascade %d %v", calls, err)
			}
			provider, _ := s.GetProvider(providerID)
			target, _ := s.GetTarget(targetID)
			if provider == nil || target == nil || provider.APIKeyRef != "env:TEST_KEY" {
				t.Fatal("configuration removed")
			}
		})
	}
	s, err := Open(filepath.Join(t.TempDir(), "invalid.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	seedHistory(t, s, 1)
	for _, f := range []HistoryFilter{{}, {All: true, BeforeMS: &before}, {IDs: []int64{-1}}, {KeepLast: &negative}} {
		if _, err := s.DeleteHistory(f, false); err == nil {
			t.Fatalf("accepted invalid filter %+v", f)
		}
	}
	p, _ := s.ListRunsPage(1)
	if p.Total != 1 {
		t.Fatal("invalid filter deleted history")
	}
}

func TestMaintenanceDoesNotInterruptRunningCalls(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	seedHistory(t, s, 1)
	maintenance, err := OpenForMaintenance(path)
	if err != nil {
		t.Fatal(err)
	}
	defer maintenance.Close()
	if _, err := maintenance.DeleteHistory(HistoryFilter{All: true}, true); err != nil {
		t.Fatal(err)
	}
	run, err := s.GetRun(1)
	if err != nil || run.Calls[0].Status != "running" {
		t.Fatalf("maintenance interrupted call: %+v %v", run, err)
	}
}
