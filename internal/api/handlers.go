package api

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	dbpkg "wasteless/internal/db"
	"wasteless/internal/store"
)

func currentMonth() string {
	return time.Now().Local().Format("2006-01")
}

func (s *server) handleConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"app":      "wasteless",
		"currency": "so'm",
	})
}

func (s *server) handleListEntries(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	month := q.Get("month")
	if month == "" {
		month = currentMonth()
	}

	limit := 0
	if raw := q.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			writeJSON(w, http.StatusBadRequest, errorBody{Error: "limit must be a non-negative integer"})
			return
		}
		limit = n
	}

	entries, err := s.store.ListEntries(store.Filter{
		Month:      month,
		Kind:       q.Get("kind"),
		WastedOnly: q.Get("wasted") == "1",
		Limit:      limit,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

func (s *server) handleCreateEntry(w http.ResponseWriter, r *http.Request) {
	var e store.Entry
	if err := decodeJSON(w, r, &e); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "invalid JSON body"})
		return
	}
	created, err := s.store.AddEntry(e)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *server) handleUpdateEntry(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "invalid id"})
		return
	}
	var e store.Entry
	if err := decodeJSON(w, r, &e); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "invalid JSON body"})
		return
	}
	e.ID = id
	updated, err := s.store.UpdateEntry(e)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *server) handleDeleteEntry(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "invalid id"})
		return
	}
	if err := s.store.DeleteEntry(id); err != nil {
		writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handleSetWasted(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "invalid id"})
		return
	}
	var body struct {
		Wasted bool `json:"wasted"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "invalid JSON body"})
		return
	}
	updated, err := s.store.SetWasted(id, body.Wasted)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *server) handleCategories(w http.ResponseWriter, r *http.Request) {
	cats, err := s.store.Categories(r.URL.Query().Get("kind"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cats)
}

func (s *server) handleSummary(w http.ResponseWriter, r *http.Request) {
	month := r.URL.Query().Get("month")
	if month == "" {
		month = currentMonth()
	}
	sum, err := s.store.MonthSummary(month)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sum)
}

func (s *server) handleChart(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("months")
	if raw == "" {
		raw = "6"
	}
	months, err := strconv.Atoi(raw)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "months must be 6 or 12"})
		return
	}
	points, err := s.store.Chart(months)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, points)
}

// handleBackup creates a fresh snapshot first; only once that succeeds do
// we set download headers and stream the file, so a failed backup never
// produces a half-written download.
func (s *server) handleBackup(w http.ResponseWriter, r *http.Request) {
	dest := filepath.Join(s.cfg.BackupDir, dbpkg.DailyName(time.Now()))
	if err := dbpkg.Backup(s.db, dest); err != nil {
		log.Printf("backup failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: "backup failed"})
		return
	}

	f, err := os.Open(dest)
	if err != nil {
		log.Printf("open backup for download: %v", err)
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: "backup failed"})
		return
	}
	defer f.Close()

	info, err := f.Stat()
	if err == nil {
		w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filepath.Base(dest)))
	io.Copy(w, f)
}
