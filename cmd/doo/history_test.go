package main

import (
	"bytes"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"doo/internal/store"
)

func TestHistoryCommand(t *testing.T) {
	previousZone := time.Local
	time.Local = time.FixedZone("UTC+8", 8*60*60)
	defer func() { time.Local = previousZone }()
	for _, tc := range []struct {
		name      string
		args      []string
		remaining int64
		output    string
	}{
		{"date preview", []string{"--before", "2026-10-09", "--dry-run"}, 4, "Would delete 2"},
		{"before local midnight", []string{"--before", "2026-10-09"}, 2, "Deleted 2"},
		{"IDs", []string{"--id", "1,3", "--id", "3,999"}, 2, "Deleted 2"},
		{"keep", []string{"--keep-last", "2"}, 2, "Deleted 2"},
		{"keep zero", []string{"--keep-last", "0"}, 0, "Deleted 4"},
		{"all", []string{"--all"}, 0, "Deleted 4"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "doo.db")
			st, err := store.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer st.Close()
			for i := 0; i < 4; i++ {
				if _, _, err := st.CreateRun("{}", "", nil); err != nil {
					t.Fatal(err)
				}
			}
			db, err := sql.Open("sqlite", path)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			cutoff, _ := time.ParseInLocation("2006-01-02", "2026-10-09", time.Local)
			for i, offset := range []int64{-86400000, -1, 0, 86400000} {
				if _, err := db.Exec("UPDATE runs SET created_at=? WHERE id=?", cutoff.UnixMilli()+offset, i+1); err != nil {
					t.Fatal(err)
				}
			}
			var out, errs bytes.Buffer
			args := append([]string{"delete", "--data", dir}, tc.args...)
			if err := runHistoryCommand(args, &out, &errs); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), tc.output) {
				t.Fatalf("output %s", out.String())
			}
			p, err := st.ListRunsPage(1)
			if err != nil || p.Total != tc.remaining {
				t.Fatalf("remaining %+v %v", p, err)
			}
			if tc.name == "before local midnight" && p.Runs[1].ID != 3 {
				t.Fatal("deleted cutoff day")
			}
		})
	}
}

func TestHistoryCommandRejectsInvalidSelectors(t *testing.T) {
	for _, args := range [][]string{{"delete"}, {"delete", "--all", "--before", "2026-10-09"}, {"delete", "--before", "2026-02-30"}, {"delete", "--before", "2026-1-1"}, {"delete", "--keep-last", "-1"}, {"delete", "--id", "-1"}, {"delete", "--id", "1,x"}, {"delete", "--all=false"}, {"delete", "--all", "extra"}} {
		var out bytes.Buffer
		if err := runHistoryCommand(args, &out, &out); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	dir := t.TempDir()
	var out bytes.Buffer
	if err := runHistoryCommand([]string{"delete", "--all", "--data", dir}, &out, &out); err == nil {
		t.Fatal("accepted missing database")
	}
}
