package handlers

import (
	"encoding/json"
	"net/http"

	"binarypanel/internal/config"
	"binarypanel/internal/services"
)

// GetEmailSettings returns current email settings
func GetEmailSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error": "method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	cfg := config.GetEmailSettings()
	
	// Mask the password before sending to frontend
	if cfg.Password != "" {
		cfg.Password = "********"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cfg)
}

// UpdateEmailSettings saves new email settings
func UpdateEmailSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, `{"error": "method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req config.EmailSettings
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "invalid json payload"}`, http.StatusBadRequest)
		return
	}

	// Prevent overwriting with masked password if they didn't change it
	current := config.GetEmailSettings()
	if req.Password == "********" {
		req.Password = current.Password
	}

	if err := config.SaveEmailSettings(req); err != nil {
		http.Error(w, `{"error": "failed to save settings"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// TestEmailSettings triggers a test email
func TestEmailSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error": "method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		To string `json:"to"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.To == "" {
		http.Error(w, `{"error": "valid 'to' email address required"}`, http.StatusBadRequest)
		return
	}

	if err := services.SendTestEmail(req.To); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}
