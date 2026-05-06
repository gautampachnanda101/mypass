package daemon

import (
	"embed"
	"html/template"
	"net/http"
	"strings"
)

//go:embed static/*
var staticFiles embed.FS

// handleWebUI serves the embedded web UI.
func (s *Server) handleWebUI(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/ui")
	if path == "" || path == "/" {
		path = "/index.html"
	}

	// Serve static files
	if strings.HasPrefix(path, "/static/") {
		content, err := staticFiles.ReadFile(strings.TrimPrefix(path, "/"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		
		// Set appropriate content type
		contentType := "text/plain"
		if strings.HasSuffix(path, ".html") {
			contentType = "text/html; charset=utf-8"
		} else if strings.HasSuffix(path, ".css") {
			contentType = "text/css; charset=utf-8"
		} else if strings.HasSuffix(path, ".js") {
			contentType = "application/javascript; charset=utf-8"
		}
		
		w.Header().Set("Content-Type", contentType)
		w.Write(content)
		return
	}

	// Serve the main index page
	if path == "/index.html" || path == "/" {
		tmpl, err := template.ParseFS(staticFiles, "static/index.html")
		if err != nil {
			http.Error(w, "Failed to load UI", http.StatusInternalServerError)
			return
		}
		
		data := struct {
			Port  int
			Token string
		}{
			Port:  s.port,
			Token: s.token,
		}
		
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		tmpl.Execute(w, data)
		return
	}

	http.NotFound(w, r)
}
