// Package api turns HTTP requests into store calls and JSON responses.
package api

import (
	"database/sql"
	"log"
	"net/http"

	"wasteless/internal/config"
	"wasteless/internal/store"
	"wasteless/web"
)

// server holds what the handlers need.
type server struct {
	store *store.Store
	db    *sql.DB
	cfg   config.Config
}

// NewRouter builds the full HTTP handler for the app: every route behind
// the localhost-only protection middleware, no CORS headers anywhere.
func NewRouter(s *store.Store, database *sql.DB, cfg config.Config) http.Handler {
	srv := &server{store: s, db: database, cfg: cfg}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", srv.handleDashboardPage)
	mux.HandleFunc("GET /entries", srv.handleEntriesPage)

	staticFS, err := web.StaticFS()
	if err != nil {
		log.Fatalf("static fs: %v", err)
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	mux.HandleFunc("GET /api/config", srv.handleConfig)
	mux.HandleFunc("GET /api/entries", srv.handleListEntries)
	mux.HandleFunc("POST /api/entries", srv.handleCreateEntry)
	mux.HandleFunc("PUT /api/entries/{id}", srv.handleUpdateEntry)
	mux.HandleFunc("DELETE /api/entries/{id}", srv.handleDeleteEntry)
	mux.HandleFunc("POST /api/entries/{id}/wasted", srv.handleSetWasted)
	mux.HandleFunc("GET /api/categories", srv.handleCategories)
	mux.HandleFunc("GET /api/summary", srv.handleSummary)
	mux.HandleFunc("GET /api/chart", srv.handleChart)
	mux.HandleFunc("GET /api/backup", srv.handleBackup)

	return protect(cfg.Port, mux)
}
