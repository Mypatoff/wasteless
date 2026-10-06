// Package store is the only place that talks SQL. It never prints and
// never touches HTTP: every method just returns (value, error), using
// sentinel errors so callers can check the reason with errors.Is.
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrInvalid  = errors.New("invalid entry")
	ErrNotFound = errors.New("not found")
)

// Kind values for an Entry.
const (
	KindSaving = "saving"
	KindSpend  = "spend"
)

const (
	minAmount = 1
	maxAmount = 100_000_000_000
	maxNote   = 200
	maxCat    = 30
	minYear   = 2000
)

var monthRe = regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])$`)

// Entry is one tracked money event.
type Entry struct {
	ID        int64  `json:"id"`
	Kind      string `json:"kind"`
	Amount    int64  `json:"amount"` // whole so'm, never a float
	Category  string `json:"category"`
	Wasted    bool   `json:"wasted"`
	Note      string `json:"note"`
	Date      string `json:"date"` // YYYY-MM-DD
	CreatedAt int64  `json:"created_at"`
}

// TODO(me): implement GetEntry(id int64) (Entry, error) and the matching
// GET /api/entries/{id} handler in internal/api. Left out on purpose as my
// exercise — the frontend doesn't need it, since the edit modal fills from
// the already-loaded list.

// Filter selects which entries ListEntries returns.
type Filter struct {
	Month      string // "YYYY-MM", or "" for all time
	Kind       string // "saving", "spend", or "" for both
	WastedOnly bool
	Limit      int // 0 means default
}

// Summary is the aggregate view for one calendar month.
type Summary struct {
	Month             string            `json:"month"`
	Saved             int64             `json:"saved"`
	Spent             int64             `json:"spent"`
	Wasted            int64             `json:"wasted"`
	WastedPermille    int64             `json:"wasted_permille"`
	ByCategory        []CategorySummary `json:"by_category"`
	TotalSavedAllTime int64             `json:"total_saved_all_time"`
	SavingCount       int64             `json:"saving_count"`
	SpendCount        int64             `json:"spend_count"`
	WastedCount       int64             `json:"wasted_count"`
}

// CategorySummary totals one spend category within a month.
type CategorySummary struct {
	Category string `json:"category"`
	Spent    int64  `json:"spent"`
	Wasted   int64  `json:"wasted"`
}

// ChartPoint is one month's totals in a multi-month chart.
type ChartPoint struct {
	Month  string `json:"month"`
	Saved  int64  `json:"saved"`
	Spent  int64  `json:"spent"`
	Wasted int64  `json:"wasted"`
}

var spendDefaults = []string{"Food", "Transport", "Games", "Subscriptions", "Shopping", "Other"}
var savingDefaults = []string{"Savings", "Other"}

// Store wraps a database connection.
type Store struct {
	db *sql.DB
}

// New wraps an already-open, already-migrated database connection.
func New(db *sql.DB) *Store {
	return &Store{db: db}
}

// normalize validates e and returns the normalized form (trimmed category,
// "Other" if it was empty, wasted cleared for savings).
func normalize(e Entry) (Entry, error) {
	if e.Kind != KindSaving && e.Kind != KindSpend {
		return Entry{}, fmt.Errorf("%w: kind must be %q or %q", ErrInvalid, KindSaving, KindSpend)
	}
	if e.Amount < minAmount || e.Amount > maxAmount {
		return Entry{}, fmt.Errorf("%w: amount must be between %d and %d", ErrInvalid, minAmount, maxAmount)
	}

	cat := strings.Join(strings.Fields(e.Category), " ")
	if cat == "" {
		cat = "Other"
	}
	if utf8.RuneCountInString(cat) > maxCat {
		return Entry{}, fmt.Errorf("%w: category must be at most %d characters", ErrInvalid, maxCat)
	}
	e.Category = cat

	if utf8.RuneCountInString(e.Note) > maxNote {
		return Entry{}, fmt.Errorf("%w: note must be at most %d characters", ErrInvalid, maxNote)
	}

	t, err := time.Parse("2006-01-02", e.Date)
	// Reformatting and comparing catches overflowed dates like 2026-02-31,
	// which time.Parse would otherwise silently normalize to March 3rd.
	if err != nil || t.Format("2006-01-02") != e.Date {
		return Entry{}, fmt.Errorf("%w: date must be a real calendar date in YYYY-MM-DD format", ErrInvalid)
	}
	if t.Year() < minYear {
		return Entry{}, fmt.Errorf("%w: date year must be %d or later", ErrInvalid, minYear)
	}
	today := time.Now().Local().Format("2006-01-02")
	if e.Date > today {
		return Entry{}, fmt.Errorf("%w: date cannot be after today", ErrInvalid)
	}

	if e.Kind == KindSaving && e.Wasted {
		return Entry{}, fmt.Errorf("%w: wasted only applies to spend entries", ErrInvalid)
	}
	return e, nil
}

// AddEntry validates and inserts an entry, returning it with its new ID
// and server-assigned created_at.
func (s *Store) AddEntry(e Entry) (Entry, error) {
	e, err := normalize(e)
	if err != nil {
		return Entry{}, err
	}
	createdAt := time.Now().Unix()

	row := s.db.QueryRow(
		`INSERT INTO entries (kind, amount, category, wasted, note, date, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		e.Kind, e.Amount, e.Category, boolToInt(e.Wasted), e.Note, e.Date, createdAt,
	)
	if err := row.Scan(&e.ID); err != nil {
		return Entry{}, fmt.Errorf("insert entry: %w", err)
	}
	e.CreatedAt = createdAt
	return e, nil
}

// UpdateEntry validates and overwrites every field of an existing entry,
// identified by e.ID. created_at is preserved from the original row.
func (s *Store) UpdateEntry(e Entry) (Entry, error) {
	// Changing kind to saving clears wasted rather than rejecting it, so
	// the edit form doesn't force the user to untick it first.
	if e.Kind == KindSaving {
		e.Wasted = false
	}
	e, err := normalize(e)
	if err != nil {
		return Entry{}, err
	}

	row := s.db.QueryRow(
		`UPDATE entries SET kind = ?, amount = ?, category = ?, wasted = ?, note = ?, date = ?
		 WHERE id = ? RETURNING created_at`,
		e.Kind, e.Amount, e.Category, boolToInt(e.Wasted), e.Note, e.Date, e.ID,
	)
	if err := row.Scan(&e.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Entry{}, ErrNotFound
		}
		return Entry{}, fmt.Errorf("update entry: %w", err)
	}
	return e, nil
}

// DeleteEntry hard-deletes an entry by ID.
func (s *Store) DeleteEntry(id int64) error {
	res, err := s.db.Exec(`DELETE FROM entries WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete entry: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete entry: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetWasted flips the wasted flag on a spend entry. It's rejected for
// saving entries, which can never be wasted.
func (s *Store) SetWasted(id int64, wasted bool) (Entry, error) {
	var kind string
	if err := s.db.QueryRow(`SELECT kind FROM entries WHERE id = ?`, id).Scan(&kind); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Entry{}, ErrNotFound
		}
		return Entry{}, fmt.Errorf("lookup entry kind: %w", err)
	}
	if kind != KindSpend {
		return Entry{}, fmt.Errorf("%w: wasted only applies to spend entries", ErrInvalid)
	}

	e := Entry{ID: id, Kind: KindSpend, Wasted: wasted}
	row := s.db.QueryRow(
		`UPDATE entries SET wasted = ? WHERE id = ?
		 RETURNING amount, category, note, date, created_at`,
		boolToInt(wasted), id,
	)
	if err := row.Scan(&e.Amount, &e.Category, &e.Note, &e.Date, &e.CreatedAt); err != nil {
		return Entry{}, fmt.Errorf("set wasted: %w", err)
	}
	return e, nil
}

// ListEntries returns entries matching filter, always as a non-nil slice.
func (s *Store) ListEntries(f Filter) ([]Entry, error) {
	var conds []string
	var args []any

	if f.Month != "" {
		start, end, err := monthRange(f.Month)
		if err != nil {
			return nil, err
		}
		conds = append(conds, "date >= ?", "date < ?")
		args = append(args, start, end)
	}

	if f.Kind != "" {
		if f.Kind != KindSaving && f.Kind != KindSpend {
			return nil, fmt.Errorf("%w: kind must be %q or %q", ErrInvalid, KindSaving, KindSpend)
		}
		conds = append(conds, "kind = ?")
		args = append(args, f.Kind)
	}

	if f.WastedOnly {
		conds = append(conds, "wasted = 1")
	}

	limit := f.Limit
	if limit <= 0 {
		limit = 200
	}
	if limit > 500 {
		limit = 500
	}

	query := `SELECT id, kind, amount, category, wasted, note, date, created_at FROM entries`
	if len(conds) > 0 {
		query += " WHERE " + strings.Join(conds, " AND ")
	}
	query += " ORDER BY date DESC, id DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list entries: %w", err)
	}
	defer rows.Close()

	entries := []Entry{}
	for rows.Next() {
		var e Entry
		var wastedInt int
		if err := rows.Scan(&e.ID, &e.Kind, &e.Amount, &e.Category, &wastedInt, &e.Note, &e.Date, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan entry: %w", err)
		}
		e.Wasted = wastedInt != 0
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list entries: %w", err)
	}
	return entries, nil
}

// Categories returns categories actually used for kind, ordered by
// frequency, followed by any default category not already in that list.
func (s *Store) Categories(kind string) ([]string, error) {
	var defaults []string
	switch kind {
	case KindSpend:
		defaults = spendDefaults
	case KindSaving:
		defaults = savingDefaults
	default:
		return nil, fmt.Errorf("%w: kind must be %q or %q", ErrInvalid, KindSaving, KindSpend)
	}

	rows, err := s.db.Query(
		`SELECT category, COUNT(*) AS c FROM entries WHERE kind = ?
		 GROUP BY category ORDER BY c DESC, category ASC`,
		kind,
	)
	if err != nil {
		return nil, fmt.Errorf("categories: %w", err)
	}
	defer rows.Close()

	seen := map[string]bool{}
	result := []string{}
	for rows.Next() {
		var cat string
		var count int
		if err := rows.Scan(&cat, &count); err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}
		if !seen[cat] {
			seen[cat] = true
			result = append(result, cat)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("categories: %w", err)
	}

	for _, cat := range defaults {
		if !seen[cat] {
			seen[cat] = true
			result = append(result, cat)
		}
	}
	return result, nil
}

// MonthSummary totals saved/spent/wasted money for one calendar month.
func (s *Store) MonthSummary(month string) (Summary, error) {
	start, end, err := monthRange(month)
	if err != nil {
		return Summary{}, err
	}

	sum := Summary{Month: month, ByCategory: []CategorySummary{}}
	err = s.db.QueryRow(
		`SELECT
			COALESCE(SUM(CASE WHEN kind = 'saving' THEN amount ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN kind = 'spend' THEN amount ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN kind = 'spend' AND wasted = 1 THEN amount ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN kind = 'saving' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN kind = 'spend' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN kind = 'spend' AND wasted = 1 THEN 1 ELSE 0 END), 0)
		 FROM entries WHERE date >= ? AND date < ?`,
		start, end,
	).Scan(&sum.Saved, &sum.Spent, &sum.Wasted, &sum.SavingCount, &sum.SpendCount, &sum.WastedCount)
	if err != nil {
		return Summary{}, fmt.Errorf("summary totals: %w", err)
	}
	if sum.Spent > 0 {
		sum.WastedPermille = sum.Wasted * 1000 / sum.Spent
	}

	if err := s.db.QueryRow(
		`SELECT COALESCE(SUM(amount), 0) FROM entries WHERE kind = 'saving'`,
	).Scan(&sum.TotalSavedAllTime); err != nil {
		return Summary{}, fmt.Errorf("summary total saved all time: %w", err)
	}

	rows, err := s.db.Query(
		`SELECT category,
			COALESCE(SUM(amount), 0),
			COALESCE(SUM(CASE WHEN wasted = 1 THEN amount ELSE 0 END), 0)
		 FROM entries WHERE kind = 'spend' AND date >= ? AND date < ?
		 GROUP BY category ORDER BY 3 DESC, 2 DESC`,
		start, end,
	)
	if err != nil {
		return Summary{}, fmt.Errorf("summary by category: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var c CategorySummary
		if err := rows.Scan(&c.Category, &c.Spent, &c.Wasted); err != nil {
			return Summary{}, fmt.Errorf("scan category summary: %w", err)
		}
		sum.ByCategory = append(sum.ByCategory, c)
	}
	if err := rows.Err(); err != nil {
		return Summary{}, fmt.Errorf("summary by category: %w", err)
	}

	return sum, nil
}

// Chart returns exactly `months` points (6 or 12), oldest first, ending
// with the current local month, zero-filled where there are no entries.
func (s *Store) Chart(months int) ([]ChartPoint, error) {
	if months != 6 && months != 12 {
		return nil, fmt.Errorf("%w: months must be 6 or 12", ErrInvalid)
	}

	now := time.Now().Local()
	anchor := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	points := make([]ChartPoint, 0, months)
	for i := months - 1; i >= 0; i-- {
		m := anchor.AddDate(0, -i, 0)
		monthStr := m.Format("2006-01")
		start := m.Format("2006-01-02")
		end := m.AddDate(0, 1, 0).Format("2006-01-02")

		p := ChartPoint{Month: monthStr}
		err := s.db.QueryRow(
			`SELECT
				COALESCE(SUM(CASE WHEN kind = 'saving' THEN amount ELSE 0 END), 0),
				COALESCE(SUM(CASE WHEN kind = 'spend' THEN amount ELSE 0 END), 0),
				COALESCE(SUM(CASE WHEN kind = 'spend' AND wasted = 1 THEN amount ELSE 0 END), 0)
			 FROM entries WHERE date >= ? AND date < ?`,
			start, end,
		).Scan(&p.Saved, &p.Spent, &p.Wasted)
		if err != nil {
			return nil, fmt.Errorf("chart month %s: %w", monthStr, err)
		}
		points = append(points, p)
	}
	return points, nil
}

// monthRange validates a "YYYY-MM" string and returns its [start, end)
// date bounds, computed with time.Date so year boundaries are handled
// correctly rather than by string manipulation on day counts.
func monthRange(month string) (start, end string, err error) {
	if !monthRe.MatchString(month) {
		return "", "", fmt.Errorf("%w: month must be in YYYY-MM format", ErrInvalid)
	}
	year, _ := strconv.Atoi(month[:4])
	mo, _ := strconv.Atoi(month[5:7])

	startDate := time.Date(year, time.Month(mo), 1, 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(0, 1, 0)
	return startDate.Format("2006-01-02"), endDate.Format("2006-01-02"), nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
