package handlers

import (
	"net/http"
)

// DashboardHandler serves the SPA frontend.
type DashboardHandler struct {
	staticDir string
}

// NewDashboardHandler creates a new dashboard handler.
func NewDashboardHandler(staticDir string) *DashboardHandler {
	return &DashboardHandler{staticDir: staticDir}
}

// ServeHTTP serves the SPA — all non-API routes serve index.html.
func (h *DashboardHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Serve static files directly if they exist
	fs := http.Dir(h.staticDir)
	f, err := fs.Open(r.URL.Path)
	if err == nil {
		f.Close()
		http.FileServer(fs).ServeHTTP(w, r)
		return
	}

	// Fall back to index.html for SPA routing
	http.ServeFile(w, r, h.staticDir+"/index.html")
}
