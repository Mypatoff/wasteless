package api

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// protect wraps every route: it only allows requests addressed to this app
// on localhost, rejects cross-origin requests, and rejects bodies that
// aren't declared as JSON. It never adds CORS headers, so no browser page
// from another origin can read these responses either.
func protect(port int, next http.Handler) http.Handler {
	allowedHosts := map[string]bool{
		fmt.Sprintf("localhost:%d", port): true,
		fmt.Sprintf("127.0.0.1:%d", port): true,
		fmt.Sprintf("[::1]:%d", port):     true,
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !allowedHosts[r.Host] {
			writeJSON(w, http.StatusForbidden, errorBody{Error: "forbidden host"})
			return
		}

		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || !allowedHosts[u.Host] {
				writeJSON(w, http.StatusForbidden, errorBody{Error: "forbidden origin"})
				return
			}
		}

		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodDelete:
			if r.ContentLength > 0 {
				ct := r.Header.Get("Content-Type")
				if !strings.HasPrefix(ct, "application/json") {
					writeJSON(w, http.StatusUnsupportedMediaType, errorBody{Error: "Content-Type must be application/json"})
					return
				}
			}
		}

		next.ServeHTTP(w, r)
	})
}
