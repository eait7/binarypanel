package handlers

import (
	"encoding/json"
	"net/http"
	"os/exec"

	"binarypanel/internal/config"
	"binarypanel/internal/services"
)

// SystemHandler provides system stats and service links.
type SystemHandler struct {
	sysinfo *services.SysInfoService
	cfg     *config.Config
}

// NewSystemHandler creates a new system handler.
func NewSystemHandler(sysinfo *services.SysInfoService, cfg *config.Config) *SystemHandler {
	return &SystemHandler{sysinfo: sysinfo, cfg: cfg}
}

// Stats handles GET /api/system/stats
func (h *SystemHandler) Stats(w http.ResponseWriter, r *http.Request) {
	stats := h.sysinfo.GetStats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// Links handles GET /api/links
func (h *SystemHandler) Links(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"filebrowser": h.cfg.FileBrowserURL,
		"portainer":   h.cfg.PortainerExternalURL,
	})
}

// UpdateSystem handles POST /api/system/update
// Securely triggers a detached daemon sequence pulling upstream GitHub alignments and reconstructing the BinaryPanel orchestrator recursively.
func (h *SystemHandler) UpdateSystem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// Dispatch organic detached background compilation sequence natively bypassing Go's lock.
	cmd := exec.Command("sh", "-c", "cd /app/host_binarypanel && git config --global --add safe.directory /app/host_binarypanel && git pull origin main && docker compose up -d --build --force-recreate dashboard &")
	if err := cmd.Start(); err != nil {
		http.Error(w, `{"error":"orchestrator sequence failed"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Update triggered successfully. Dashboard will detach and completely reset organically in ~30 seconds.",
	})
}
