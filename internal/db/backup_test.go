package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func seedRow(t *testing.T, database *sql.DB) {
	t.Helper()
	_, err := database.Exec(
		`INSERT INTO entries (kind, amount, category, wasted, note, date, created_at)
		 VALUES ('spend', 100, 'Food', 0, '', '2026-01-01', 1)`,
	)
	if err != nil {
		t.Fatalf("seed row: %v", err)
	}
}

func TestBackupOpensWithSameRows(t *testing.T) {
	dir := t.TempDir()
	database, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()
	seedRow(t, database)

	dest := filepath.Join(dir, "backup.db")
	if err := Backup(database, dest); err != nil {
		t.Fatalf("Backup: %v", err)
	}

	copyDB, err := sql.Open("sqlite", dest)
	if err != nil {
		t.Fatalf("open backup copy: %v", err)
	}
	defer copyDB.Close()

	var count int
	if err := copyDB.QueryRow(`SELECT COUNT(*) FROM entries`).Scan(&count); err != nil {
		t.Fatalf("query backup copy: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 row in backup, got %d", count)
	}
}

func TestEnsureStartBackupNotOverwritten(t *testing.T) {
	dir := t.TempDir()
	dbDir := t.TempDir()
	database, err := Open(filepath.Join(dbDir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()
	seedRow(t, database)

	if err := EnsureStartBackup(database, dir); err != nil {
		t.Fatalf("EnsureStartBackup first call: %v", err)
	}
	startPath := filepath.Join(dir, StartName(time.Now()))
	info1, err := os.Stat(startPath)
	if err != nil {
		t.Fatalf("stat start backup: %v", err)
	}

	// Add a second row, then ensure-start again: the file must not change.
	if _, err := database.Exec(
		`INSERT INTO entries (kind, amount, category, wasted, note, date, created_at)
		 VALUES ('spend', 200, 'Food', 0, '', '2026-01-02', 2)`,
	); err != nil {
		t.Fatalf("insert second row: %v", err)
	}
	if err := EnsureStartBackup(database, dir); err != nil {
		t.Fatalf("EnsureStartBackup second call: %v", err)
	}
	info2, err := os.Stat(startPath)
	if err != nil {
		t.Fatalf("stat start backup again: %v", err)
	}
	if info1.ModTime() != info2.ModTime() || info1.Size() != info2.Size() {
		t.Fatalf("start backup was overwritten: before=%+v after=%+v", info1, info2)
	}

	copyDB, err := sql.Open("sqlite", startPath)
	if err != nil {
		t.Fatalf("open start backup: %v", err)
	}
	defer copyDB.Close()
	var count int
	if err := copyDB.QueryRow(`SELECT COUNT(*) FROM entries`).Scan(&count); err != nil {
		t.Fatalf("query start backup: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected start backup to still have 1 row, got %d", count)
	}
}

func TestRotateDeletesOnlyOldDatedFiles(t *testing.T) {
	dir := t.TempDir()
	old := time.Now().AddDate(0, 0, -40)
	recent := time.Now().AddDate(0, 0, -5)

	files := map[string]bool{ // name -> expected to survive
		DailyName(old):          false,
		StartName(old):          false,
		DailyName(recent):       true,
		StartName(recent):       true,
		"not-a-backup-file.db":  true,
		"wasteless-notadate.db": true,
	}
	for name := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	if err := Rotate(dir, 30); err != nil {
		t.Fatalf("Rotate: %v", err)
	}

	for name, shouldSurvive := range files {
		_, err := os.Stat(filepath.Join(dir, name))
		exists := err == nil
		if exists != shouldSurvive {
			t.Errorf("%s: exists=%v, want %v", name, exists, shouldSurvive)
		}
	}
}
