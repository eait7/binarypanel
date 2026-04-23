package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"time"
)

// AppsHandler manages 1-Click deployments.
type AppsHandler struct{}

// NewAppsHandler creates a new apps handler.
func NewAppsHandler() *AppsHandler {
	return &AppsHandler{}
}

// DeployBinaryCMS handles POST /api/apps/deploy/binarycms
func (h *AppsHandler) DeployBinaryCMS(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Port string `json:"port"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Port == "" {
		http.Error(w, `{"error":"port is required"}`, http.StatusBadRequest)
		return
	}

	containerName := fmt.Sprintf("binarycms_%d", time.Now().Unix())

	// Step 1: Tell Docker to build the image natively straight from GitHub.
	buildCmd := exec.Command("docker", "build", "-t", "eait7/binarycms:latest", "https://github.com/eait7/BinaryCMS.git#main")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Build Failed: " + string(out)})
		return
	}

	// Step 2: Spawn the container attached to the binarypanel network securely.
	// We map the public port and persist data blocks natively.
	runCmd := exec.Command("docker", "run", "-d", 
		"--name", containerName,
		"--network", "binarypanel_binarypanel",
		"-p", fmt.Sprintf("%s:8080", req.Port),
		"-v", fmt.Sprintf("%s_uploads:/app/uploads", containerName),
		"-v", fmt.Sprintf("%s_db:/app/data", containerName),
		"--restart", "unless-stopped",
		"eait7/binarycms:latest")

	if out, err := runCmd.CombinedOutput(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Deploy Failed: " + string(out)})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "BinaryCMS successfully deployed securely on Port " + req.Port,
		"container": containerName,
	})
}
