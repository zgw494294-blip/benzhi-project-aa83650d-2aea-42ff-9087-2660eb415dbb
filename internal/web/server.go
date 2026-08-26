package web

import (
	"net/http"
	"time"

	"phonemereleasedesk/internal/application"
)

type Server struct {
	service *application.Service
	mux     *http.ServeMux
}

func New(service *application.Service) *Server {
	s := &Server{service: service, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return securityHeaders(requestLog(s.mux))
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /", s.WorkbenchHandler)
	s.mux.HandleFunc("GET /assets/app.css", s.StylesHandler)
	s.mux.HandleFunc("GET /assets/app.js", s.ScriptHandler)
	s.mux.HandleFunc("GET /api/health", s.HealthHandler)
	s.mux.HandleFunc("GET /api/batches", s.ListBatchesHandler)
	s.mux.HandleFunc("POST /api/batches", s.CreateBatchHandler)
	s.mux.HandleFunc("GET /api/batches/{id}", s.GetBatchHandler)
	s.mux.HandleFunc("PUT /api/batches/{id}/spec", s.UpdateSpecificationHandler)
	s.mux.HandleFunc("POST /api/batches/{id}/segments", s.AddSegmentHandler)
	s.mux.HandleFunc("POST /api/batches/{id}/segments/preflight", s.PreflightSegmentsHandler)
	s.mux.HandleFunc("POST /api/batches/{id}/segments/bulk", s.AddSegmentsHandler)
	s.mux.HandleFunc("DELETE /api/batches/{id}/segments/{segment}", s.RemoveSegmentHandler)
	s.mux.HandleFunc("POST /api/batches/{id}/freeze", s.FreezeHandler)
	s.mux.HandleFunc("POST /api/batches/{id}/assignments", s.AssignHandler)
	s.mux.HandleFunc("POST /api/batches/{id}/assignments/preview", s.PreviewAssignmentsHandler)
	s.mux.HandleFunc("POST /api/batches/{id}/assignments/bulk", s.AssignManyHandler)
	s.mux.HandleFunc("POST /api/batches/{id}/submissions", s.SubmitAnnotationHandler)
	s.mux.HandleFunc("GET /api/batches/{id}/submissions/{segment}", s.OwnSubmissionHandler)
	s.mux.HandleFunc("POST /api/batches/{id}/checks", s.RunCheckHandler)
	s.mux.HandleFunc("GET /api/batches/{id}/checks/history", s.VerificationHistoryHandler)
	s.mux.HandleFunc("GET /api/batches/{id}/checks/compare", s.CompareChecksHandler)
	s.mux.HandleFunc("POST /api/batches/{id}/decisions", s.DecideHandler)
	s.mux.HandleFunc("GET /api/batches/{id}/decisions", s.DecisionQueueHandler)
	s.mux.HandleFunc("POST /api/batches/{id}/decisions/bulk", s.DecideManyHandler)
	s.mux.HandleFunc("POST /api/batches/{id}/repairs", s.RepairHandler)
	s.mux.HandleFunc("GET /api/batches/{id}/repairs/tasks", s.RepairTasksHandler)
	s.mux.HandleFunc("POST /api/batches/{id}/seal", s.SealHandler)
	s.mux.HandleFunc("GET /api/credentials/{id}/verify", s.VerifyCredentialHandler)
	s.mux.HandleFunc("GET /api/credentials", s.CredentialDetailHandler)
	s.mux.HandleFunc("GET /api/credentials/verify", s.VerifyCredentialDetailHandler)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'; connect-src 'self'")
		next.ServeHTTP(w, r)
	})
}

func requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		_ = start
	})
}
