package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"binarypanel/internal/services"
)

// ContainersHandler manages Docker containers.
type ContainersHandler struct {
	docker *services.DockerService
}

// NewContainersHandler creates a new containers handler.
func NewContainersHandler(docker *services.DockerService) *ContainersHandler {
	return &ContainersHandler{docker: docker}
}

// List handles GET /api/containers
func (h *ContainersHandler) List(w http.ResponseWriter, r *http.Request) {
	containers, err := h.docker.ListContainers()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"containers": containers,
	})
}

// Start handles POST /api/containers/{id}/start
func (h *ContainersHandler) Start(w http.ResponseWriter, r *http.Request) {
	id := extractContainerID(r.URL.Path)
	if id == "" {
		http.Error(w, `{"error":"missing container id"}`, http.StatusBadRequest)
		return
	}

	if err := h.docker.StartContainer(id); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

// Stop handles POST /api/containers/{id}/stop
func (h *ContainersHandler) Stop(w http.ResponseWriter, r *http.Request) {
	id := extractContainerID(r.URL.Path)
	if id == "" {
		http.Error(w, `{"error":"missing container id"}`, http.StatusBadRequest)
		return
	}

	if err := h.docker.StopContainer(id); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

// Restart handles POST /api/containers/{id}/restart
func (h *ContainersHandler) Restart(w http.ResponseWriter, r *http.Request) {
	id := extractContainerID(r.URL.Path)
	if id == "" {
		http.Error(w, `{"error":"missing container id"}`, http.StatusBadRequest)
		return
	}

	if err := h.docker.RestartContainer(id); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}


// Logs handles GET /api/containers/{id}/logs
func (h *ContainersHandler) Logs(w http.ResponseWriter, r *http.Request) {
	id := extractContainerID(r.URL.Path)
	if id == "" {
		http.Error(w, `{"error":"missing container id"}`, http.StatusBadRequest)
		return
	}

	lines := 100
	if l := r.URL.Query().Get("lines"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			lines = n
		}
	}

	logs, err := h.docker.GetContainerLogs(id, lines)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"container_id": id,
		"logs":         logs,
	})
}

// extractContainerID extracts the container ID from a URL path like /api/containers/{id}/action
func extractContainerID(path string) string {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	// Expected: api, containers, {id}, {action}
	for i, p := range parts {
		if p == "containers" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}
