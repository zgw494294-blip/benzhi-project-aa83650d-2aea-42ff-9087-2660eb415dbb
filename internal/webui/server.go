package webui

import (
	"net/http"
	"strings"
	"time"

	"acousticverdictworkbench/internal/application"
)

type Server struct{ service *application.Service }

func New(service *application.Service) *Server { return &Server{service: service} }

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.PageHandler)
	mux.HandleFunc("GET /static/", s.StaticHandler)
	mux.HandleFunc("GET /api/health", s.HealthHandler)
	mux.HandleFunc("GET /api/batches", s.ListBatchesHandler)
	mux.HandleFunc("POST /api/batches", s.CreateBatchHandler)
	mux.HandleFunc("GET /api/batches/{batchID}", s.GetBatchHandler)
	mux.HandleFunc("PUT /api/batches/{batchID}/scope", s.ConfigureScopeHandler)
	mux.HandleFunc("POST /api/batches/{batchID}/clips", s.AddClipHandler)
	mux.HandleFunc("POST /api/batches/{batchID}/clips/bulk", s.BulkRegisterClipsHandler)
	mux.HandleFunc("DELETE /api/batches/{batchID}/clips/{clipID}", s.RemoveClipHandler)
	mux.HandleFunc("POST /api/batches/{batchID}/freeze", s.FreezeBatchHandler)
	mux.HandleFunc("PUT /api/batches/{batchID}/clips/{clipID}/draft", s.SaveDraftHandler)
	mux.HandleFunc("POST /api/batches/{batchID}/clips/{clipID}/submit", s.SubmitAnnotationHandler)
	mux.HandleFunc("POST /api/batches/{batchID}/disputes/{disputeID}/resolve", s.ResolveDisputeHandler)
	mux.HandleFunc("POST /api/batches/{batchID}/quality", s.QualityCheckHandler)
	mux.HandleFunc("POST /api/batches/{batchID}/release", s.ReleaseBatchHandler)
	mux.HandleFunc("GET /api/batches/{batchID}/manifest", s.ManifestDetailsHandler)
	mux.HandleFunc("GET /api/batches/{batchID}/manifest/verify", s.VerifyManifestHandler)
	return securityHeaders(mux)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) HealthHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "time": time.Now().UTC()})
}

func actor(r *http.Request) (string, string) {
	return strings.TrimSpace(r.URL.Query().Get("actorId")), strings.TrimSpace(r.URL.Query().Get("role"))
}
