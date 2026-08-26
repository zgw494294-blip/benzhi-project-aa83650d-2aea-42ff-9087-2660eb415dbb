package web

import (
	"embed"
	"net/http"
)

//go:embed static/index.html static/app.css static/app.js
var assets embed.FS

func serveAsset(w http.ResponseWriter, path, contentType string) {
	data, err := assets.ReadFile(path)
	if err != nil {
		http.NotFound(w, nil)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) WorkbenchHandler(w http.ResponseWriter, _ *http.Request) {
	serveAsset(w, "static/index.html", "text/html; charset=utf-8")
}
func (s *Server) StylesHandler(w http.ResponseWriter, _ *http.Request) {
	serveAsset(w, "static/app.css", "text/css; charset=utf-8")
}
func (s *Server) ScriptHandler(w http.ResponseWriter, _ *http.Request) {
	serveAsset(w, "static/app.js", "text/javascript; charset=utf-8")
}
func (s *Server) HealthHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
