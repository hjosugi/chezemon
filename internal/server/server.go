package server

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/hjosugi/chezemon/internal/state"
	chezweb "github.com/hjosugi/chezemon/internal/web"
)

type Server struct {
	service *state.Service
	logger  *slog.Logger
}

func New(service *state.Service, logger *slog.Logger) http.Handler {
	server := &Server{service: service, logger: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/healthz", server.health)
	mux.HandleFunc("GET /api/snapshot", server.snapshot)
	mux.HandleFunc("GET /api/diff", server.diff)
	mux.HandleFunc("GET /api/doctor", server.doctor)

	assets, err := fs.Sub(chezweb.Assets, ".")
	if err != nil {
		panic(err)
	}
	mux.Handle("/", http.FileServer(http.FS(assets)))
	return securityHeaders(requestLog(logger, mux))
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "readOnly": true})
}

func (s *Server) snapshot(w http.ResponseWriter, r *http.Request) {
	force, _ := strconv.ParseBool(r.URL.Query().Get("force"))
	snapshot, err := s.service.Snapshot(r.Context(), force)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) diff(w http.ResponseWriter, r *http.Request) {
	target := r.URL.Query().Get("path")
	if target == "" {
		writeError(w, http.StatusBadRequest, errors.New("path is required"))
		return
	}
	reveal, _ := strconv.ParseBool(r.URL.Query().Get("reveal"))
	diff, err := s.service.Diff(r.Context(), target, reveal)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, diff)
}

func (s *Server) doctor(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.service.Doctor(r.Context()))
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]any{"error": err.Error()})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'none'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

func requestLog(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.Debug("request", "method", r.Method, "path", r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
