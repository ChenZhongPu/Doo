package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"doo/internal/store"
)

func runHistoryCommand(args []string, out, errOut io.Writer) error {
	if len(args) == 0 || args[0] != "delete" {
		fmt.Fprintln(errOut, "Usage: doo history delete --all | --before YYYY-MM-DD | --id ID[,ID...] | --keep-last N [--data DIR] [--dry-run]")
		if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
			return flag.ErrHelp
		}
		return fmt.Errorf("expected the delete subcommand")
	}
	fs := flag.NewFlagSet("doo history delete", flag.ContinueOnError)
	fs.SetOutput(errOut)
	all := fs.Bool("all", false, "Delete all run history")
	before := fs.String("before", "", "Delete runs before YYYY-MM-DD (exclusive, local time)")
	keep := fs.Int64("keep-last", 0, "Keep the latest N runs and delete older ones")
	dryRun := fs.Bool("dry-run", false, "Show matching run count without deleting")
	data := fs.String("data", "~/.doo", "Data directory containing doo.db")
	var ids []int64
	fs.Func("id", "Delete run IDs (comma-separated; may be repeated)", func(value string) error {
		for _, raw := range strings.Split(value, ",") {
			id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
			if err != nil || id <= 0 {
				return fmt.Errorf("invalid run ID %q", raw)
			}
			ids = append(ids, id)
		}
		return nil
	})
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	filter := store.HistoryFilter{All: *all, IDs: ids}
	selectors := 0
	keepSpecified := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "keep-last" {
			keepSpecified = true
		}
		if f.Name == "all" || f.Name == "before" || f.Name == "id" || f.Name == "keep-last" {
			selectors++
		}
	})
	if selectors != 1 || *keep < 0 || (!*all && *before == "" && len(ids) == 0 && !keepSpecified) {
		return fmt.Errorf("choose exactly one of --all, --before, --id, or --keep-last (N >= 0)")
	}
	if *before != "" {
		date, err := time.ParseInLocation("2006-01-02", *before, time.Local)
		if err != nil {
			return fmt.Errorf("invalid --before date: use YYYY-MM-DD")
		}
		ms := date.UnixMilli()
		filter.BeforeMS = &ms
	}
	if keepSpecified {
		filter.KeepLast = keep
	}
	dbPath := filepath.Join(expandPath(*data), "doo.db")
	if _, err := os.Stat(dbPath); err != nil {
		return fmt.Errorf("open history database: %w", err)
	}
	st, err := store.OpenForMaintenance(dbPath)
	if err != nil {
		return err
	}
	defer st.Close()
	count, err := st.DeleteHistory(filter, *dryRun)
	if err != nil {
		return err
	}
	if *dryRun {
		fmt.Fprintf(out, "Would delete %d run(s). No history was deleted.\n", count)
	} else {
		fmt.Fprintf(out, "Deleted %d run(s).\n", count)
	}
	return nil
}
