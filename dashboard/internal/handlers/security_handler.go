package handlers

import (
	"encoding/json"
	"net/http"

	"binarypanel/internal/services"
)

// SecurityHandler manages security configuration API endpoints.
type SecurityHandler struct {
	security *services.SecurityService
	caddy    *services.CaddyService
}

// NewSecurityHandler creates a new security handler.
func NewSecurityHandler(security *services.SecurityService, caddy *services.CaddyService) *SecurityHandler {
	return &SecurityHandler{security: security, caddy: caddy}
}

// DomainSecurityStatus represents the security status returned per domain.
type DomainSecurityStatus struct {
	Domain         string `json:"domain"`
	SecurityScore  int    `json:"security_score"`
	HeadersEnabled bool   `json:"headers_enabled"`
	IPRestricted   bool   `json:"ip_restricted"`
	HSTSEnabled    bool   `json:"hsts_enabled"`
	CSPEnabled     bool   `json:"csp_enabled"`
	XFrameSet      bool   `json:"x_frame_set"`
}

// ListDomainSecurity handles GET /api/security/domains — returns security status for all domains.
func (h *SecurityHandler) ListDomainSecurity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// Get all domains from Caddy
	domains, err := h.caddy.ListDomains()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	allConfigs := h.security.GetAllDomainConfigs()

	var statuses []DomainSecurityStatus
	for _, d := range domains {
		domainName := ""
		if len(d.Domains) > 0 {
			domainName = d.Domains[0]
		}
		if domainName == "" {
			continue
		}

		cfg, hasConfig := allConfigs[domainName]
		status := DomainSecurityStatus{
			Domain: domainName,
		}

		if hasConfig {
			status.SecurityScore = h.security.CalculateScore(cfg)
			status.HeadersEnabled = cfg.HSTSEnabled || cfg.XFrameOptions != "" || cfg.XContentTypeOpts || cfg.ReferrerPolicy != "" || cfg.CSPEnabled
			status.IPRestricted = len(cfg.IPWhitelist) > 0 || len(cfg.IPBlacklist) > 0
			status.HSTSEnabled = cfg.HSTSEnabled
			status.CSPEnabled = cfg.CSPEnabled
			status.XFrameSet = cfg.XFrameOptions != ""
		}

		statuses = append(statuses, status)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"domains": statuses,
	})
}

// GetDomainSecurity handles GET /api/security/domain?name=example.com
func (h *SecurityHandler) GetDomainSecurity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	domain := r.URL.Query().Get("name")
	if domain == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "missing 'name' query parameter"})
		return
	}

	cfg := h.security.GetDomainConfig(domain)
	score := h.security.CalculateScore(cfg)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"config": cfg,
		"score":  score,
	})
}

// UpdateDomainSecurity handles PUT /api/security/domain — save + apply security config.
func (h *SecurityHandler) UpdateDomainSecurity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var cfg services.DomainSecurityConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid request body: " + err.Error()})
		return
	}

	if cfg.Domain == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "domain name is required"})
		return
	}

	// Set default HSTS max-age if not provided
	if cfg.HSTSEnabled && cfg.HSTSMaxAge <= 0 {
		cfg.HSTSMaxAge = 31536000
	}

	if err := h.security.SetDomainConfig(cfg); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	score := h.security.CalculateScore(cfg)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Security configuration applied successfully",
		"score":   score,
	})
}

// ListBlockedIPs handles GET /api/security/ips
func (h *SecurityHandler) ListBlockedIPs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	ips := h.security.GetBlockedIPs()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"blocked_ips": ips,
		"total":       len(ips),
	})
}

// BlockIP handles POST /api/security/ips/block
func (h *SecurityHandler) BlockIP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		IP     string `json:"ip"`
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid request body"})
		return
	}

	if req.IP == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "IP address is required"})
		return
	}

	if err := h.security.BlockIP(req.IP, req.Reason); err != nil {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "IP " + req.IP + " has been blocked globally",
	})
}

// UnblockIP handles DELETE /api/security/ips/unblock
func (h *SecurityHandler) UnblockIP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		IP string `json:"ip"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid request body"})
		return
	}

	if req.IP == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "IP address is required"})
		return
	}

	if err := h.security.UnblockIP(req.IP); err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "IP " + req.IP + " has been unblocked",
	})
}
