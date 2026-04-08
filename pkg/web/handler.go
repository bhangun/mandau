// Package web provides HTTP handlers for the Mandau web dashboard
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"path"

	"github.com/bhangun/mandau/pkg/api"
	"github.com/bhangun/mandau/pkg/auth"
)

//go:embed dashboard.html static
var staticFiles embed.FS

// CoreInterface defines the interface for core operations
type CoreInterface interface {
	api.CoreInterface
}

// Handler returns an HTTP handler that serves the web dashboard and REST API
func Handler(c CoreInterface, authMW *auth.Middleware) http.Handler {
	mux := http.NewServeMux()

	// Serve static files
	staticFS, _ := fs.Sub(staticFiles, "static")
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	// Register REST API routes if core is provided
	if c != nil {
		apiHandler := api.NewHandler(c, authMW)
		apiHandler.RegisterRoutes(mux)
	}

	// Serve dashboard.html at root
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		content, err := staticFiles.ReadFile("dashboard.html")
		if err != nil {
			http.Error(w, "Failed to load dashboard", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(content)
	})

	// Serve login.html
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/login" {
			http.NotFound(w, r)
			return
		}

		content, err := staticFiles.ReadFile("login.html")
		if err != nil {
			http.Error(w, "Failed to load login page", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(content)
	})

	return mux
}

// ServeMux returns a configured ServeMux for embedding in existing servers
func ServeMux(c CoreInterface, authMW *auth.Middleware) *http.ServeMux {
	return Handler(c, authMW).(*http.ServeMux)
}

// GetFile serves a specific file from the embedded filesystem
func GetFile(filepath string) ([]byte, error) {
	return staticFiles.ReadFile(path.Clean(filepath))
}
