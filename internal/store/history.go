package store

import "fmt"

const HistoryPageSize = 100

type RunPage struct {
	Runs     []*RunSummary `json:"runs"`
	Total    int64         `json:"total"`
	Page     int64         `json:"page"`
	PageSize int           `json:"page_size"`
}

// ListRunsPage reads the count and summaries from the same database snapshot.
// Clamp to the last page when deletions have shortened the history.
func (s *Store) ListRunsPage(page int64) (*RunPage, error) {
	if page < 1 {
		return nil, fmt.Errorf("page must be positive")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	result := &RunPage{PageSize: HistoryPageSize}
	if err := tx.QueryRow("SELECT COUNT(*) FROM runs").Scan(&result.Total); err != nil {
		return nil, err
	}
	lastPage := (result.Total + HistoryPageSize - 1) / HistoryPageSize
	if lastPage < 1 {
		lastPage = 1
	}
	if page > lastPage {
		page = lastPage
	}
	result.Page = page
	result.Runs, err = listRunSummaries(tx,
		"SELECT id, created_at, input_json, note FROM runs ORDER BY id DESC LIMIT ? OFFSET ?",
		HistoryPageSize, (page-1)*HistoryPageSize)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

// Exactly one selector is required; a zero-value filter never deletes anything.
type HistoryFilter struct {
	All      bool
	BeforeMS *int64
	IDs      []int64
	KeepLast *int64
}

func (f HistoryFilter) predicate() (string, []any, error) {
	selectors := 0
	var where string
	var args []any
	if f.All {
		selectors++
		where = "1 = 1"
	}
	if f.BeforeMS != nil {
		selectors++
		where, args = "created_at < ?", []any{*f.BeforeMS}
	}
	if len(f.IDs) > 0 {
		selectors++
		for _, id := range f.IDs {
			if id <= 0 {
				return "", nil, fmt.Errorf("run IDs must be positive")
			}
		}
		where, args = "id IN ("+makePlaceholders(len(f.IDs))+")", toAnySlice(f.IDs)
	}
	if f.KeepLast != nil {
		selectors++
		if *f.KeepLast < 0 {
			return "", nil, fmt.Errorf("keep-last must not be negative")
		}
		where = "id IN (SELECT id FROM runs ORDER BY id DESC LIMIT -1 OFFSET ?)"
		args = []any{*f.KeepLast}
	}
	if selectors != 1 {
		return "", nil, fmt.Errorf("choose exactly one of all, before, IDs, or keep-last")
	}
	return where, args, nil
}

// DeleteHistory cascades to calls, preserving providers, targets, and keys.
func (s *Store) DeleteHistory(filter HistoryFilter, dryRun bool) (int64, error) {
	where, args, err := filter.predicate()
	if err != nil {
		return 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if dryRun {
		var count int64
		err := s.db.QueryRow("SELECT COUNT(*) FROM runs WHERE "+where, args...).Scan(&count)
		return count, err
	}
	res, err := s.db.Exec("DELETE FROM runs WHERE "+where, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
