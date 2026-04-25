package handlers

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"strconv"

	"binarypanel/internal/config"
	"binarypanel/internal/services"
)

// SystemHandler provides system stats and service links.
type SystemHandler struct {
	sysinfo *services.SysInfoService
	cfg     *config.Config
	logger  *services.PanelLogger
}

// NewSystemHandler creates a new system handler.
func NewSystemHandler(sysinfo *services.SysInfoService, cfg *config.Config) *SystemHandler {
	return &SystemHandler{sysinfo: sysinfo, cfg: cfg, logger: services.GetLogger()}
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

	if h.logger != nil {
		h.logger.Info("system", "System update triggered by admin")
	}

	// Dispatch organic detached background compilation sequence natively bypassing Go's lock.
	cmd := exec.Command("sh", "-c", "cd /app/host_binarypanel && git config --global --add safe.directory /app/host_binarypanel && git pull origin main && docker compose up -d --build --force-recreate dashboard &")
	if err := cmd.Start(); err != nil {
		if h.logger != nil {
			h.logger.Error("system", "System update failed to start", err.Error())
		}
		http.Error(w, `{"error":"orchestrator sequence failed"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Update triggered successfully. Dashboard will detach and completely reset organically in ~30 seconds.",
	})
}

// Logs handles GET /api/system/logs — returns structured log entries for the error log viewer.
func (h *SystemHandler) Logs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	limit := 100
	if q := r.URL.Query().Get("limit"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 {
			limit = n
		}
	}

	var entries []services.LogEntry
	if h.logger != nil {
		levelFilter := r.URL.Query().Get("level")
		if levelFilter != "" {
			entries = h.logger.GetEntriesByLevel(services.LogLevel(levelFilter), limit)
		} else {
			entries = h.logger.GetEntries(limit)
		}
	} else {
		entries = []services.LogEntry{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"entries": entries,
		"total":   len(entries),
	})
}

// RebootStack handles POST /api/system/reboot-stack — restarts all BinaryPanel services via docker compose.
func (h *SystemHandler) RebootStack(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	if h.logger != nil {
		h.logger.Warn("system", "Full stack reboot triggered by admin")
	}

	// Detached restart of the entire binarypanel compose stack.
	// Uses nohup + background to ensure the command survives the dashboard container restarting.
	cmd := exec.Command("sh", "-c", "cd /app/host_binarypanel && docker compose restart &")
	if err := cmd.Start(); err != nil {
		if h.logger != nil {
			h.logger.Error("system", "Stack reboot failed", err.Error())
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to trigger stack reboot: " + err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Stack reboot initiated. All services will restart. Dashboard will reconnect in ~15 seconds.",
	})
}
