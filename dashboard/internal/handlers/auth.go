package handlers

import (
	"encoding/json"
	"net/http"

	"golang.org/x/crypto/bcrypt"
	
	"binarypanel/internal/config"
	"binarypanel/internal/middleware"
)

// AuthHandler handles login/logout/session endpoints.
type AuthHandler struct {
	cfg  *config.Config
	auth *middleware.Auth
}

// NewAuthHandler creates a new auth handler.
func NewAuthHandler(cfg *config.Config, auth *middleware.Auth) *AuthHandler {
	return &AuthHandler{cfg: cfg, auth: auth}
}

// Login handles POST /api/auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var creds struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	authConfig := config.GetAuthSettings()
	validUser := authConfig.Username
	if validUser == "" {
		validUser = h.cfg.Username
	}

	valid := false
	if authConfig.Password != "" {
		err := bcrypt.CompareHashAndPassword([]byte(authConfig.Password), []byte(creds.Password))
		valid = (err == nil && creds.Username == validUser)
	} else {
		// Fallback to .env setup globally intrinsically
		valid = (creds.Username == validUser && creds.Password == h.cfg.Password)
	}

	if !valid {
		http.Error(w, `{"error":"invalid credentials"}`, http.StatusUnauthorized)
		return
	}

	token := h.auth.GenerateToken(creds.Username)
	http.SetCookie(w, &http.Cookie{
		Name:     "binarypanel_session",
		Value:    token,
		Path:     "/",
		MaxAge:   86400,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"username": creds.Username,
	})
}

// Session handles GET /api/auth/session
func (h *AuthHandler) Session(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("binarypanel_session")
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"authenticated": false,
		})
		return
	}

	username, valid := h.auth.ValidateToken(cookie.Value)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"authenticated": valid,
		"username":      username,
	})
}

// Logout handles POST /api/auth/logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "binarypanel_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
	})
}

// UpdateCredentials dynamically rewrites the core login structures natively
func (h *AuthHandler) UpdateCredentials(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var creds struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(creds.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, `{"error":"encryption error"}`, http.StatusInternalServerError)
		return
	}

	authData := config.GetAuthSettings()
	authData.Username = creds.Username
	authData.Password = string(hash)

	if err := config.SaveAuthSettings(authData); err != nil {
		http.Error(w, `{"error":"save failure"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}
