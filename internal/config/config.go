// Package config resolves runtime settings from the environment.
package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config holds every setting the server needs to start.
type Config struct {
	Port           int
	DBPath         string // absolute path to the SQLite file
	BackupDir      string // absolute path to the backup folder
	BackupKeepDays int
	NoBrowser      bool
}

// Load reads environment variables, applying defaults suited to a
// double-clicked single-user app on someone's own laptop.
func Load() (Config, error) {
	cfg := Config{
		Port:           8080,
		BackupKeepDays: 30,
	}

	if v := os.Getenv("PORT"); v != "" {
		p, err := strconv.Atoi(v)
		if err != nil {
			p = 8080
		}
		cfg.Port = p
	}

	if v := os.Getenv("BACKUP_KEEP_DAYS"); v != "" {
		d, err := strconv.Atoi(v)
		if err == nil {
			cfg.BackupKeepDays = d
		}
	}

	cfg.NoBrowser = os.Getenv("NO_BROWSER") == "1"

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = defaultDBPath()
	}
	abs, err := filepath.Abs(dbPath)
	if err != nil {
		return Config{}, err
	}
	cfg.DBPath = abs

	backupDir := os.Getenv("BACKUP_DIR")
	if backupDir == "" {
		backupDir = filepath.Join(filepath.Dir(cfg.DBPath), "backups")
	}
	absBackup, err := filepath.Abs(backupDir)
	if err != nil {
		return Config{}, err
	}
	cfg.BackupDir = absBackup

	return cfg, nil
}

// defaultDBPath puts the database next to the running executable, unless
// that executable looks like a throwaway `go run` build (its path contains
// "go-build", the per-invocation scratch directory `go run` compiles
// into) — there, we'd be writing into a folder that's about to be wiped,
// so fall back to the current working directory instead. A real binary
// that merely happens to live under the OS temp dir (e.g. copied there
// for a quick test) is left alone: it's not ephemeral just because of
// where it's sitting.
func defaultDBPath() string {
	exe, err := os.Executable()
	if err != nil {
		return "wasteless.db"
	}
	if strings.Contains(exe, "go-build") {
		return "wasteless.db"
	}
	return filepath.Join(filepath.Dir(exe), "wasteless.db")
}
