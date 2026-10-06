// Command server runs the WasteLess app: a tiny local tracker for money
// saved vs. money wasted, single user, no login.
package main

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"wasteless/internal/api"
	"wasteless/internal/config"
	dbpkg "wasteless/internal/db"
	"wasteless/internal/store"
)

const backupInterval = 6 * time.Hour

func main() {
	cfg, err := config.Load()
	if err != nil {
		fatalStartup(fmt.Errorf("load config: %w", err))
	}

	addr := fmt.Sprintf("127.0.0.1:%d", cfg.Port)

	// Listen FIRST, before touching the database: if something else is
	// already listening, we want to find out before we've opened anything
	// that would need cleaning up.
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		handleBusyPort(cfg)
		return
	}

	database, err := dbpkg.Open(cfg.DBPath)
	if err != nil {
		fatalStartup(fmt.Errorf("open database: %w", err))
	}
	defer database.Close()

	if err := dbpkg.EnsureStartBackup(database, cfg.BackupDir); err != nil {
		log.Printf("startup backup failed: %v", err)
	}
	if err := dbpkg.Rotate(cfg.BackupDir, cfg.BackupKeepDays); err != nil {
		log.Printf("backup rotation failed: %v", err)
	}

	s := store.New(database)
	handler := api.NewRouter(s, database, cfg)

	srv := &http.Server{
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	url := "http://" + addr + "/"
	fmt.Printf("WasteLess listening on %s\n", url)
	fmt.Printf("Database: %s\n", cfg.DBPath)
	fmt.Printf("Backups:  %s\n", cfg.BackupDir)

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()

	if !cfg.NoBrowser {
		openBrowser(url)
	}

	stopBackups := make(chan struct{})
	go runBackupLoop(database, cfg, stopBackups)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serveErr:
		if err != nil && err != http.ErrServerClosed {
			log.Printf("server error: %v", err)
		}
	case <-sig:
		fmt.Println("shutting down...")
	}

	close(stopBackups)
	finalBackup(database, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}

// handleBusyPort runs when we couldn't bind the port ourselves. If another
// WasteLess instance already owns it, we just hand off to that one;
// otherwise we tell the user clearly and give them time to read it.
func handleBusyPort(cfg config.Config) {
	url := fmt.Sprintf("http://127.0.0.1:%d/", cfg.Port)
	if isWasteless(cfg.Port) {
		fmt.Printf("WasteLess is already running at %s — opening it in your browser.\n", url)
		if !cfg.NoBrowser {
			openBrowser(url)
		}
		os.Exit(0)
	}
	fmt.Printf("port %d is used by another program; set PORT=... to use a different one.\n", cfg.Port)
	waitForEnterOrTimeout(10 * time.Second)
	os.Exit(1)
}

// isWasteless asks whatever is listening on port whether it's us.
func isWasteless(port int) bool {
	client := &http.Client{Timeout: time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/api/config", port))
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return false
	}
	app, _ := body["app"].(string)
	return app == "wasteless"
}

// fatalStartup prints a startup error clearly and gives a double-clicked
// window time to be read before the process disappears.
func fatalStartup(err error) {
	fmt.Fprintf(os.Stderr, "WasteLess failed to start:\n  %v\n\nPress Enter to exit, or this window will close automatically in 20s.\n", err)
	waitForEnterOrTimeout(20 * time.Second)
	os.Exit(1)
}

func waitForEnterOrTimeout(d time.Duration) {
	done := make(chan struct{})
	go func() {
		bufio.NewReader(os.Stdin).ReadString('\n')
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(d):
	}
}

// runBackupLoop rewrites today's snapshot and rotates old ones every
// backupInterval, until stop is closed.
func runBackupLoop(database *sql.DB, cfg config.Config, stop <-chan struct{}) {
	ticker := time.NewTicker(backupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			dest := filepath.Join(cfg.BackupDir, dbpkg.DailyName(time.Now()))
			if err := dbpkg.Backup(database, dest); err != nil {
				log.Printf("periodic backup failed: %v", err)
			}
			if err := dbpkg.Rotate(cfg.BackupDir, cfg.BackupKeepDays); err != nil {
				log.Printf("backup rotation failed: %v", err)
			}
		case <-stop:
			return
		}
	}
}

func finalBackup(database *sql.DB, cfg config.Config) {
	dest := filepath.Join(cfg.BackupDir, dbpkg.DailyName(time.Now()))
	if err := dbpkg.Backup(database, dest); err != nil {
		log.Printf("shutdown backup failed: %v", err)
	}
	if err := dbpkg.Rotate(cfg.BackupDir, cfg.BackupKeepDays); err != nil {
		log.Printf("backup rotation failed: %v", err)
	}
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		log.Printf("could not open browser: %v", err)
	}
}
