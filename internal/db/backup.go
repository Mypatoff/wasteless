package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// DailyName is the rewritten-every-6-hours snapshot name for day t.
func DailyName(t time.Time) string {
	return fmt.Sprintf("wasteless-%s.db", t.Format("2006-01-02"))
}

// StartName is the once-per-day "as the app was when it started" snapshot.
func StartName(t time.Time) string {
	return fmt.Sprintf("wasteless-%s-start.db", t.Format("2006-01-02"))
}

// backupFileRe matches both snapshot name shapes, capturing the date, so
// Rotate can tell dated backups apart from any other file in the folder.
var backupFileRe = regexp.MustCompile(`^wasteless-(\d{4}-\d{2}-\d{2})(?:-start)?\.db$`)

// Backup writes a consistent snapshot of database to dest. It refuses to
// touch dest if the database fails its integrity check, so a corrupt live
// database can never clobber a good backup.
func Backup(database *sql.DB, dest string) error {
	var result string
	if err := database.QueryRow(`PRAGMA quick_check`).Scan(&result); err != nil {
		return fmt.Errorf("quick_check: %w", err)
	}
	if result != "ok" {
		log.Printf("backup aborted: database failed quick_check: %s", result)
		return fmt.Errorf("database failed quick_check: %s", result)
	}

	if dir := filepath.Dir(dest); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create backup directory: %w", err)
		}
	}

	tmp := dest + ".tmp"
	os.Remove(tmp) // clear any leftover from a previous failed attempt

	if _, err := database.Exec(`VACUUM INTO ?`, tmp); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("vacuum into: %w", err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("rename backup into place: %w", err)
	}
	return nil
}

// EnsureStartBackup creates today's start-of-day snapshot the first time
// the app starts today, and leaves it alone on every later restart.
func EnsureStartBackup(database *sql.DB, dir string) error {
	dest := filepath.Join(dir, StartName(time.Now()))
	if _, err := os.Stat(dest); err == nil {
		return nil // already have today's start snapshot; never overwrite it
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat start backup: %w", err)
	}
	return Backup(database, dest)
}

// Rotate deletes dated backup files older than keepDays, ignoring any file
// in dir that doesn't match the wasteless backup naming pattern.
func Rotate(dir string, keepDays int) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read backup dir: %w", err)
	}

	cutoff := time.Now().AddDate(0, 0, -keepDays)
	for _, entry := range entries {
		m := backupFileRe.FindStringSubmatch(entry.Name())
		if m == nil {
			continue
		}
		day, err := time.Parse("2006-01-02", m[1])
		if err != nil {
			continue
		}
		if day.Before(cutoff) {
			if err := os.Remove(filepath.Join(dir, entry.Name())); err != nil {
				log.Printf("rotate: could not remove %s: %v", entry.Name(), err)
			}
		}
	}
	return nil
}
