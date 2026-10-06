package store

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"wasteless/internal/db"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return New(database)
}

func validSpend() Entry {
	return Entry{Kind: KindSpend, Amount: 1000, Category: "Food", Date: "2026-01-15"}
}

func TestAddUpdateDeleteListHappyPath(t *testing.T) {
	s := newTestStore(t)

	created, err := s.AddEntry(validSpend())
	if err != nil {
		t.Fatalf("AddEntry: %v", err)
	}
	if created.ID == 0 || created.CreatedAt == 0 {
		t.Fatalf("expected assigned id/created_at, got %+v", created)
	}

	updated := created
	updated.Amount = 2000
	updated.Note = "updated"
	got, err := s.UpdateEntry(updated)
	if err != nil {
		t.Fatalf("UpdateEntry: %v", err)
	}
	if got.Amount != 2000 || got.Note != "updated" || got.CreatedAt != created.CreatedAt {
		t.Fatalf("update not applied correctly: %+v", got)
	}

	list, err := s.ListEntries(Filter{})
	if err != nil {
		t.Fatalf("ListEntries: %v", err)
	}
	if len(list) != 1 || list[0].ID != created.ID {
		t.Fatalf("expected 1 entry, got %+v", list)
	}

	if err := s.DeleteEntry(created.ID); err != nil {
		t.Fatalf("DeleteEntry: %v", err)
	}
	list, err = s.ListEntries(Filter{})
	if err != nil {
		t.Fatalf("ListEntries after delete: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected empty list, got %+v", list)
	}
}

func TestAmountValidation(t *testing.T) {
	s := newTestStore(t)
	cases := []int64{0, -1, 100_000_000_001}
	for _, amount := range cases {
		e := validSpend()
		e.Amount = amount
		if _, err := s.AddEntry(e); !errors.Is(err, ErrInvalid) {
			t.Errorf("amount %d: expected ErrInvalid, got %v", amount, err)
		}
	}
}

func TestDateValidation(t *testing.T) {
	s := newTestStore(t)

	future := validSpend()
	future.Date = time.Now().Local().AddDate(0, 0, 1).Format("2006-01-02")
	if _, err := s.AddEntry(future); !errors.Is(err, ErrInvalid) {
		t.Errorf("future date: expected ErrInvalid, got %v", err)
	}

	bad := validSpend()
	bad.Date = "2026-02-31"
	if _, err := s.AddEntry(bad); !errors.Is(err, ErrInvalid) {
		t.Errorf("2026-02-31: expected ErrInvalid, got %v", err)
	}
}

func TestWastedOnlyAllowedForSpend(t *testing.T) {
	s := newTestStore(t)

	saving := Entry{Kind: KindSaving, Amount: 500, Category: "Savings", Date: "2026-01-10", Wasted: true}
	if _, err := s.AddEntry(saving); !errors.Is(err, ErrInvalid) {
		t.Errorf("wasted saving: expected ErrInvalid, got %v", err)
	}
}

func TestUpdateSpendToSavingClearsWasted(t *testing.T) {
	s := newTestStore(t)

	spend := validSpend()
	spend.Wasted = true
	created, err := s.AddEntry(spend)
	if err != nil {
		t.Fatalf("AddEntry: %v", err)
	}

	toSaving := created
	toSaving.Kind = KindSaving
	toSaving.Category = "Savings"
	toSaving.Wasted = true // should be force-cleared, not rejected
	got, err := s.UpdateEntry(toSaving)
	if err != nil {
		t.Fatalf("UpdateEntry to saving: %v", err)
	}
	if got.Wasted {
		t.Fatalf("expected wasted cleared after switching to saving, got %+v", got)
	}
}

func TestSetWastedRejectsSaving(t *testing.T) {
	s := newTestStore(t)

	saving, err := s.AddEntry(Entry{Kind: KindSaving, Amount: 500, Category: "Savings", Date: "2026-01-10"})
	if err != nil {
		t.Fatalf("AddEntry: %v", err)
	}
	if _, err := s.SetWasted(saving.ID, true); !errors.Is(err, ErrInvalid) {
		t.Errorf("SetWasted on saving: expected ErrInvalid, got %v", err)
	}

	spend, err := s.AddEntry(validSpend())
	if err != nil {
		t.Fatalf("AddEntry: %v", err)
	}
	got, err := s.SetWasted(spend.ID, true)
	if err != nil {
		t.Fatalf("SetWasted on spend: %v", err)
	}
	if !got.Wasted {
		t.Fatalf("expected wasted=true, got %+v", got)
	}
}

func TestMonthBoundaries(t *testing.T) {
	s := newTestStore(t)

	jan31 := validSpend()
	jan31.Date = "2026-01-31"
	if _, err := s.AddEntry(jan31); err != nil {
		t.Fatalf("AddEntry jan31: %v", err)
	}
	feb01 := validSpend()
	feb01.Date = "2026-02-01"
	if _, err := s.AddEntry(feb01); err != nil {
		t.Fatalf("AddEntry feb01: %v", err)
	}

	janList, err := s.ListEntries(Filter{Month: "2026-01"})
	if err != nil {
		t.Fatalf("ListEntries jan: %v", err)
	}
	if len(janList) != 1 || janList[0].Date != "2026-01-31" {
		t.Fatalf("expected only jan31 in January, got %+v", janList)
	}

	febList, err := s.ListEntries(Filter{Month: "2026-02"})
	if err != nil {
		t.Fatalf("ListEntries feb: %v", err)
	}
	if len(febList) != 1 || febList[0].Date != "2026-02-01" {
		t.Fatalf("expected only feb01 in February, got %+v", febList)
	}

	dec := validSpend()
	dec.Date = "2025-12-31"
	if _, err := s.AddEntry(dec); err != nil {
		t.Fatalf("AddEntry dec31: %v", err)
	}
	decList, err := s.ListEntries(Filter{Month: "2025-12"})
	if err != nil {
		t.Fatalf("ListEntries dec: %v", err)
	}
	if len(decList) != 1 {
		t.Fatalf("expected only dec31 in December, got %+v", decList)
	}
}

func TestMonthSummary(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.AddEntry(Entry{Kind: KindSaving, Amount: 1000, Category: "Savings", Date: "2026-01-05"}); err != nil {
		t.Fatalf("AddEntry saving: %v", err)
	}
	if _, err := s.AddEntry(Entry{Kind: KindSpend, Amount: 300, Category: "Food", Date: "2026-01-06", Wasted: true}); err != nil {
		t.Fatalf("AddEntry spend1: %v", err)
	}
	if _, err := s.AddEntry(Entry{Kind: KindSpend, Amount: 700, Category: "Food", Date: "2026-01-07"}); err != nil {
		t.Fatalf("AddEntry spend2: %v", err)
	}

	sum, err := s.MonthSummary("2026-01")
	if err != nil {
		t.Fatalf("MonthSummary: %v", err)
	}
	if sum.Saved != 1000 || sum.Spent != 1000 || sum.Wasted != 300 {
		t.Fatalf("unexpected totals: %+v", sum)
	}
	if sum.WastedPermille != 300 { // 300/1000*1000
		t.Fatalf("unexpected permille: %d", sum.WastedPermille)
	}

	empty, err := s.MonthSummary("2026-03")
	if err != nil {
		t.Fatalf("MonthSummary empty: %v", err)
	}
	if empty.Spent != 0 || empty.WastedPermille != 0 {
		t.Fatalf("expected zeroed summary with spent=0, got %+v", empty)
	}
}

func TestChartZeroFillsAndCrossesYearBoundary(t *testing.T) {
	s := newTestStore(t)
	points, err := s.Chart(6)
	if err != nil {
		t.Fatalf("Chart: %v", err)
	}
	if len(points) != 6 {
		t.Fatalf("expected 6 points, got %d", len(points))
	}
	for _, p := range points {
		if p.Saved != 0 || p.Spent != 0 || p.Wasted != 0 {
			t.Fatalf("expected zero-filled point, got %+v", p)
		}
	}

	now := time.Now().Local()
	anchor := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	firstMonth := anchor.AddDate(0, -5, 0)
	lastMonth := anchor

	if points[0].Month != firstMonth.Format("2006-01") {
		t.Fatalf("expected first point %s, got %s", firstMonth.Format("2006-01"), points[0].Month)
	}
	if points[5].Month != lastMonth.Format("2006-01") {
		t.Fatalf("expected last point %s, got %s", lastMonth.Format("2006-01"), points[5].Month)
	}

	if _, err := s.Chart(1); !errors.Is(err, ErrInvalid) {
		t.Errorf("Chart(1): expected ErrInvalid, got %v", err)
	}
}

func TestUpdateDeleteUnknownID(t *testing.T) {
	s := newTestStore(t)

	e := validSpend()
	e.ID = 9999
	if _, err := s.UpdateEntry(e); !errors.Is(err, ErrNotFound) {
		t.Errorf("update unknown: expected ErrNotFound, got %v", err)
	}
	if err := s.DeleteEntry(9999); !errors.Is(err, ErrNotFound) {
		t.Errorf("delete unknown: expected ErrNotFound, got %v", err)
	}
}

func TestBadMonth(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.MonthSummary("2026-13"); !errors.Is(err, ErrInvalid) {
		t.Errorf("month 2026-13: expected ErrInvalid, got %v", err)
	}
	if _, err := s.ListEntries(Filter{Month: "not-a-month"}); !errors.Is(err, ErrInvalid) {
		t.Errorf("bad filter month: expected ErrInvalid, got %v", err)
	}
}
