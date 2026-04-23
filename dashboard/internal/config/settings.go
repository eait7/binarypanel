package config

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// EmailSettings structures the SMTP configuration
type EmailSettings struct {
	Provider string `json:"provider"`
	Host     string `json:"host"`
	Port     string `json:"port"` // Keep as string for flexibility (e.g. "587")
	Username string `json:"username"`
	Password string `json:"password"`
	From     string `json:"from"`
	Secure   bool   `json:"secure"` // whether to use SSL directly (465) or STARTTLS (587)
}

// AuthSettings manages persistent UI credentials securely
type AuthSettings struct {
	Username string `json:"username"`
	Password string `json:"password_hash"` // Hashed via bcrypt
}

// AppSettings represents the persistent JSON settings
type AppSettings struct {
	Email EmailSettings `json:"email"`
	Auth  AuthSettings  `json:"auth"`
}

var (
	settingsFile = "/data/settings.json"
	settingsMu   sync.RWMutex
	current      AppSettings
)

// InitSettings initializes the settings file, creating it if it doesn't exist
func InitSettings() {
	if _, err := os.Stat("/data"); os.IsNotExist(err) {
		log.Println("WARNING: /data directory not found, settings will not be persisted.")
		// Fallback for local testing outside Docker
		settingsFile = "settings.json"
	}

	settingsMu.Lock()
	defer settingsMu.Unlock()

	// Try reading
	data, err := os.ReadFile(settingsFile)
	if err == nil {
		if err := json.Unmarshal(data, &current); err != nil {
			log.Printf("Error parsing settings: %v", err)
		}
	} else {
		// Create default
		saveLocked(current)
	}
}

// GetEmailSettings returns a copy of the current email config
func GetEmailSettings() EmailSettings {
	settingsMu.RLock()
	defer settingsMu.RUnlock()
	return current.Email
}

// SaveEmailSettings updates and persists the email configuration
func SaveEmailSettings(es EmailSettings) error {
	settingsMu.Lock()
	defer settingsMu.Unlock()

	current.Email = es
	return saveLocked(current)
}

// GetAuthSettings returns a copy of the current auth config
func GetAuthSettings() AuthSettings {
	settingsMu.RLock()
	defer settingsMu.RUnlock()
	return current.Auth
}

// SaveAuthSettings updates and persists the auth configuration
func SaveAuthSettings(as AuthSettings) error {
	settingsMu.Lock()
	defer settingsMu.Unlock()

	current.Auth = as
	return saveLocked(current)
}

// saveLocked writes the settings to disk (assumes lock is held)
func saveLocked(s AppSettings) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	// Ensure dir exists if using fallback
	if dir := filepath.Dir(settingsFile); dir != "." {
		os.MkdirAll(dir, 0755)
	}

	return os.WriteFile(settingsFile, data, 0600)
}
