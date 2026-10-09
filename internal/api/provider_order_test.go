package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"doo/internal/store"
)

func TestAPI_MoveProvider(t *testing.T) {
	srv, st, ks := setupTestServer(t)
	defer st.Close()
	key, err := ks.Encrypt("private-provider-key")
	if err != nil {
		t.Fatal(err)
	}
	var ids []int64
	for _, name := range []string{"Alpha", "Beta"} {
		id, err := st.CreateProvider(&store.ProviderRecord{Name: name, Protocol: "openai_chat", BaseURL: "http://localhost/v1", APIKeyRef: key})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	handler := srv.Handler()
	for _, tc := range []struct {
		name, id, body string
		status         int
	}{
		{"invalid ID", "abc", `{"direction":"up"}`, 400},
		{"negative ID", "-1", `{"direction":"up"}`, 400},
		{"invalid body", "1", `{`, 400},
		{"invalid direction", "1", `{"direction":"left"}`, 400},
		{"missing provider", "999", `{"direction":"up"}`, 404},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "http://127.0.0.1:8787/api/providers/"+tc.id+"/move", strings.NewReader(tc.body))
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			if w.Code != tc.status {
				t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
			}
		})
	}
	req := httptest.NewRequest("POST", fmt.Sprintf("http://127.0.0.1:8787/api/providers/%d/move", ids[1]), strings.NewReader(`{"direction":"up"}`))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("move status = %d: %s", w.Code, w.Body.String())
	}
	var providers []ProviderResponse
	if err := json.Unmarshal(w.Body.Bytes(), &providers); err != nil {
		t.Fatal(err)
	}
	if len(providers) != 2 || providers[0].ID != ids[1] || providers[1].ID != ids[0] {
		t.Fatalf("unexpected order: %v", providers)
	}
	for _, provider := range providers {
		if provider.APIKeyRef != "enc:***" {
			t.Fatalf("move response exposes encrypted key reference: %q", provider.APIKeyRef)
		}
	}
	// Cross-origin move requests must be rejected before changing the order.
	req = httptest.NewRequest("POST", fmt.Sprintf("http://127.0.0.1:8787/api/providers/%d/move", ids[1]), strings.NewReader(`{"direction":"down"}`))
	req.Header.Set("Origin", "http://malicious.invalid")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("cross-origin move status = %d", w.Code)
	}
	got, err := st.ListProviders()
	if err != nil || got[0].ID != ids[1] {
		t.Fatalf("rejected request changed provider order: %v, %v", got, err)
	}
}
