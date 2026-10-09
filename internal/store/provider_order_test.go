package store

import (
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
)

func TestProviderOrderMigrationAndPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	legacySQL, err := migrationsFS.ReadFile("migrations/0001_init.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(legacySQL)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`PRAGMA user_version = 1;
		INSERT INTO providers(id, name, protocol, base_url, api_key_ref, headers_json, stream_usage, created_at, updated_at)
		VALUES (2, 'Alpha', 'openai_chat', 'http://localhost/v1', 'enc:kept', '{"X-Test":"kept"}', 1, 100, 200),
		       (7, 'Beta', 'openai_chat', 'http://localhost/v1', 'none', '{}', 1, 100, 200),
		       (12, 'Gamma', 'anthropic', 'http://localhost/v1', 'none', '{}', 0, 100, 200);
		INSERT INTO targets(provider_id, label, model_name, created_at, updated_at) VALUES(2, 'Existing model', 'example', 100, 200);`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	assertOrder := func(want ...string) {
		t.Helper()
		providers, err := s.ListProviders()
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, provider := range providers {
			names = append(names, provider.Name)
		}
		if !reflect.DeepEqual(names, want) {
			t.Fatalf("order = %v, want %v", names, want)
		}
	}
	assertOrder("Alpha", "Beta", "Gamma")
	p, err := s.GetProvider(2)
	if err != nil || p.APIKeyRef != "enc:kept" || p.Headers["X-Test"] != "kept" || p.UpdatedAt != 200 {
		t.Fatalf("migration changed existing settings: %v, %v", p, err)
	}
	targets, err := s.ListTargets()
	if err != nil || len(targets) != 1 || targets[0].ProviderID != 2 {
		t.Fatalf("migration changed existing targets: %v, %v", targets, err)
	}
	for _, move := range []struct {
		id        int64
		direction string
	}{
		{12, "up"}, {12, "up"}, {12, "up"}, {7, "down"},
	} {
		if err := s.MoveProvider(move.id, move.direction); err != nil {
			t.Fatal(err)
		}
	}
	assertOrder("Gamma", "Alpha", "Beta")
	if err := s.MoveProvider(999, "up"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing provider error = %v", err)
	}
	if err := s.MoveProvider(12, "invalid"); err == nil {
		t.Fatal("invalid direction accepted")
	}
	assertOrder("Gamma", "Alpha", "Beta")
	if err := s.MoveProvider(12, "down"); err != nil {
		t.Fatal(err)
	}
	assertOrder("Alpha", "Gamma", "Beta")
	if err := s.UpdateProvider(p); err != nil {
		t.Fatal(err)
	}
	assertOrder("Alpha", "Gamma", "Beta")
	// The oldest ID is moved to the end; a new provider must still append.
	if err := s.MoveProvider(2, "down"); err != nil {
		t.Fatal(err)
	}
	if err := s.MoveProvider(2, "down"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateProvider(&ProviderRecord{Name: "Delta", Protocol: "openai_chat", BaseURL: "http://localhost/v1", APIKeyRef: "none"}); err != nil {
		t.Fatal(err)
	}
	assertOrder("Gamma", "Beta", "Alpha", "Delta")
	if err := s.DeleteProvider(7); err != nil {
		t.Fatal(err)
	}
	assertOrder("Gamma", "Alpha", "Delta")
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	assertOrder("Gamma", "Alpha", "Delta")
	var version int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != 2 {
		t.Fatalf("schema version = %d, error = %v", version, err)
	}
}
