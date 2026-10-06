package api

import (
	"log"
	"net/http"

	"wasteless/web"
)

func (s *server) handleDashboardPage(w http.ResponseWriter, r *http.Request) {
	renderPage(w, "dashboard")
}

func (s *server) handleEntriesPage(w http.ResponseWriter, r *http.Request) {
	renderPage(w, "entries")
}

func renderPage(w http.ResponseWriter, name string) {
	tmpl, err := web.Page(name)
	if err != nil {
		log.Printf("parse page %s: %v", name, err)
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: "template error"})
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "layout.html", map[string]string{"Active": name}); err != nil {
		log.Printf("render page %s: %v", name, err)
	}
}
