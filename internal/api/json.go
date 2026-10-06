package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"wasteless/internal/store"
)

// maxBodyBytes caps request bodies; a single entry is tiny.
const maxBodyBytes = 1 << 16

type errorBody struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeStoreError maps a store sentinel error to the right HTTP status,
// logging the detail for anything unexpected rather than leaking it.
func writeStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrInvalid):
		writeJSON(w, http.StatusBadRequest, errorBody{Error: err.Error()})
	case errors.Is(err, store.ErrNotFound):
		writeJSON(w, http.StatusNotFound, errorBody{Error: "not found"})
	default:
		log.Printf("internal error: %v", err)
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: "internal error"})
	}
}

// decodeJSON reads a JSON body with a size limit, rejecting unknown
// fields and anything left over after the first JSON value.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	return dec.Decode(v)
}
