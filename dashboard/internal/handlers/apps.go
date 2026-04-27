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

	// Execute deployment asynchronously so the UI doesn't hang waiting for the build
	go func() {
		// Step 1: Tell Docker to build the image natively straight from GitHub.
		buildCmd := exec.Command("docker", "build", "-t", "eait7/binarycms:latest", "https://github.com/eait7/BinaryCMS.git#main")
		if err := buildCmd.Run(); err != nil {
			return // In a production system, we would log this to a file
		}

		// Step 2: Spawn the container attached to the binarypanel network securely.
		runCmd := exec.Command("docker", "run", "-d", 
			"--name", containerName,
			"--network", "binarypanel_binarypanel",
			"-p", fmt.Sprintf("%s:8080", req.Port),
			"-v", fmt.Sprintf("%s_uploads:/app/uploads", containerName),
			"-v", fmt.Sprintf("%s_db:/app/data", containerName),
			"-v", fmt.Sprintf("%s_themes:/app/themes", containerName),
			"-v", fmt.Sprintf("%s_plugins:/app/plugins", containerName),
			"-v", fmt.Sprintf("%s_plugins_data:/app/plugins_data", containerName),
			"--restart", "unless-stopped",
			"eait7/binarycms:latest")
		
		runCmd.Run()
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Deployment securely initiated! It is compiling natively in the background and will be available on Port " + req.Port + " in ~60 seconds.",
		"container": containerName,
	})
}
