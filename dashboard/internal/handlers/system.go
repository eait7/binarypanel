package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"strings"

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
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	stats := h.sysinfo.GetStats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// Links handles GET /api/links
func (h *SystemHandler) Links(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
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

	// Capture the current commit hash before pulling for the audit log.
	preHash, _ := exec.Command("sh", "-c",
		"cd /app/host_binarypanel && git rev-parse --short HEAD 2>/dev/null").Output()

	// Pull latest code from GitHub and fully recreate all containers.
	// force-recreate ensures new network config, IPs and env vars always apply.
	cmd := exec.Command("sh", "-c",
		"cd /app/host_binarypanel && "+
			"git config --global --add safe.directory /app/host_binarypanel && "+
			"git pull origin main && "+
			"docker compose up -d --build --force-recreate &")
	if err := cmd.Start(); err != nil {
		if h.logger != nil {
			h.logger.Error("system", "System update failed to start", err.Error())
		}
		http.Error(w, `{"error":"orchestrator sequence failed"}`, http.StatusInternalServerError)
		return
	}

	if h.logger != nil {
		h.logger.Info("system", fmt.Sprintf("Update initiated from commit %s — rebuild in progress",
			strings.TrimSpace(string(preHash))))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Update started. BinaryPanel is pulling the latest version and restarting all services. This takes about 45 seconds — the page will reconnect automatically.",
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

	// Full recreate of the entire binarypanel compose stack.
	// Uses force-recreate so all config changes (IPs, ports, env vars) are applied.
	cmd := exec.Command("sh", "-c", "cd /app/host_binarypanel && docker compose up -d --force-recreate &")
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
