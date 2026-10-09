package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"doo/internal/store"
)

func TestHistoryPagingAndDeleteAll(t *testing.T) {
	srv, st, _ := setupTestServer(t)
	defer st.Close()
	for i := 0; i < 205; i++ {
		if _, _, err := st.CreateRun("{}", "", nil); err != nil {
			t.Fatal(err)
		}
	}
	handler := srv.Handler()
	request := func(method, query, origin string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://127.0.0.1:8787/api/runs"+query, nil)
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	for _, q := range []string{"?page=0", "?page=-1", "?page=abc", "?page=", "?page=1&before=10"} {
		if w := request("GET", q, ""); w.Code != 400 {
			t.Fatalf("%s: %d %s", q, w.Code, w.Body.String())
		}
	}
	for _, tc := range []struct {
		query string
		page  int64
		count int
	}{{"?page=1", 1, 100}, {"?page=2&limit=999", 2, 100}, {"?page=999999999999", 3, 5}} {
		w := request("GET", tc.query, "")
		if w.Code != 200 {
			t.Fatal(w.Body.String())
		}
		var p store.RunPage
		if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		if p.Page != tc.page || len(p.Runs) != tc.count || p.Total != 205 || p.PageSize != 100 {
			t.Fatalf("unexpected page %+v", p)
		}
	}
	w := request("GET", "?limit=1000", "")
	var legacy []store.RunSummary
	if err := json.Unmarshal(w.Body.Bytes(), &legacy); err != nil || len(legacy) != 100 {
		t.Fatalf("legacy cap: %d %v", len(legacy), err)
	}
	if w := request("DELETE", "?all=true", "http://evil.invalid"); w.Code != 403 {
		t.Fatal("cross-origin deletion allowed")
	}
	for _, q := range []string{"", "?all=false", "?before=2026-10-01", "?all=true&before=2026-10-01"} {
		if w := request("DELETE", q, ""); w.Code != 400 {
			t.Fatalf("unqualified deletion allowed: %s", q)
		}
	}
	p, _ := st.ListRunsPage(1)
	if p.Total != 205 {
		t.Fatal("rejected request deleted history")
	}
	w = request("DELETE", "?all=true", "")
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var result struct {
		Deleted int64 `json:"deleted"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || result.Deleted != 205 {
		t.Fatalf("wrong deleted count %+v %v", result, err)
	}
	p, _ = st.ListRunsPage(1)
	if p.Total != 0 || p.Page != 1 {
		t.Fatalf("history remains %+v", p)
	}
	w = request("DELETE", "?all=true", "")
	if w.Code != 200 {
		t.Fatal("empty history deletion failed")
	}
}
