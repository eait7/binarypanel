package services

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DomainSecurityConfig holds the security configuration for a single domain.
type DomainSecurityConfig struct {
	Domain string `json:"domain"`

	// HSTS
	HSTSEnabled    bool `json:"hsts_enabled"`
	HSTSMaxAge     int  `json:"hsts_max_age"`     // seconds, default 31536000
	HSTSSubdomains bool `json:"hsts_subdomains"`  // includeSubDomains
	HSTSPreload    bool `json:"hsts_preload"`     // preload directive

	// Security Headers
	XFrameOptions    string `json:"x_frame_options"`    // "DENY", "SAMEORIGIN", or ""
	XContentTypeOpts bool   `json:"x_content_type_opts"` // X-Content-Type-Options: nosniff
	ReferrerPolicy   string `json:"referrer_policy"`     // "strict-origin-when-cross-origin" etc.
	PermissionsPolicy string `json:"permissions_policy"` // e.g. "camera=(), microphone=()"

	// Content Security Policy
	CSPEnabled bool   `json:"csp_enabled"`
	CSPValue   string `json:"csp_value"` // raw CSP directive value

	// IP Access Control
	IPWhitelist []string `json:"ip_whitelist"` // If set, only these IPs can access
	IPBlacklist []string `json:"ip_blacklist"` // These IPs are blocked
}

// BlockedIP represents a globally blocked IP address.
type BlockedIP struct {
	IP        string `json:"ip"`
	Reason    string `json:"reason"`
	BlockedAt string `json:"blocked_at"`
}

// GlobalSecurityConfig holds all security configuration.
type GlobalSecurityConfig struct {
	BlockedIPs []BlockedIP                      `json:"blocked_ips"`
	Domains    map[string]DomainSecurityConfig   `json:"domains"`
}

// SecurityService manages security configuration persistence and Caddy integration.
type SecurityService struct {
	mu       sync.RWMutex
	config   GlobalSecurityConfig
	filePath string
	caddy    *CaddyService
}

// NewSecurityService creates a new security service.
func NewSecurityService(filePath string, caddy *CaddyService) *SecurityService {
	svc := &SecurityService{
		filePath: filePath,
		caddy:    caddy,
		config: GlobalSecurityConfig{
			BlockedIPs: []BlockedIP{},
			Domains:    make(map[string]DomainSecurityConfig),
		},
	}
	svc.loadConfig()
	return svc
}

// loadConfig reads security configuration from disk.
func (s *SecurityService) loadConfig() {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		// File doesn't exist yet, use defaults
		s.saveConfigLocked()
		return
	}
	if err := json.Unmarshal(data, &s.config); err != nil {
		if logger := GetLogger(); logger != nil {
			logger.Error("security", "Failed to parse security config: "+err.Error())
		}
	}
	if s.config.Domains == nil {
		s.config.Domains = make(map[string]DomainSecurityConfig)
	}
	if s.config.BlockedIPs == nil {
		s.config.BlockedIPs = []BlockedIP{}
	}
}

// saveConfigLocked writes security configuration to disk (caller must hold lock or be in init).
func (s *SecurityService) saveConfigLocked() error {
	data, err := json.MarshalIndent(s.config, "", "  ")
	if err != nil {
		return err
	}
	if dir := filepath.Dir(s.filePath); dir != "." {
		os.MkdirAll(dir, 0755)
	}
	return os.WriteFile(s.filePath, data, 0600)
}

// SaveConfig persists the current config to disk (thread-safe).
func (s *SecurityService) SaveConfig() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveConfigLocked()
}

// GetDomainConfig returns the security config for a specific domain.
func (s *SecurityService) GetDomainConfig(domain string) DomainSecurityConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if cfg, ok := s.config.Domains[domain]; ok {
		return cfg
	}
	// Return defaults
	return DomainSecurityConfig{
		Domain:     domain,
		HSTSMaxAge: 31536000,
	}
}

// SetDomainConfig saves a domain's security config and applies it to Caddy.
func (s *SecurityService) SetDomainConfig(cfg DomainSecurityConfig) error {
	s.mu.Lock()
	s.config.Domains[cfg.Domain] = cfg
	if err := s.saveConfigLocked(); err != nil {
		s.mu.Unlock()
		return fmt.Errorf("failed to save security config: %w", err)
	}
	s.mu.Unlock()

	// Apply to Caddy
	if err := s.applyDomainToCaddy(cfg); err != nil {
		if logger := GetLogger(); logger != nil {
			logger.Error("security", fmt.Sprintf("Failed to apply security config for %s: %v", cfg.Domain, err))
		}
		return err
	}

	if logger := GetLogger(); logger != nil {
		logger.Info("security", fmt.Sprintf("Security config applied for %s (score: %d)", cfg.Domain, s.CalculateScore(cfg)))
	}
	return nil
}

// GetAllDomainConfigs returns all saved domain security configurations.
func (s *SecurityService) GetAllDomainConfigs() map[string]DomainSecurityConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]DomainSecurityConfig, len(s.config.Domains))
	for k, v := range s.config.Domains {
		result[k] = v
	}
	return result
}

// GetBlockedIPs returns the list of globally blocked IPs.
func (s *SecurityService) GetBlockedIPs() []BlockedIP {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]BlockedIP, len(s.config.BlockedIPs))
	copy(result, s.config.BlockedIPs)
	return result
}

// BlockIP adds an IP to the global blocklist.
func (s *SecurityService) BlockIP(ip, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for duplicates
	for _, b := range s.config.BlockedIPs {
		if b.IP == ip {
			return fmt.Errorf("IP %s is already blocked", ip)
		}
	}

	s.config.BlockedIPs = append(s.config.BlockedIPs, BlockedIP{
		IP:        ip,
		Reason:    reason,
		BlockedAt: time.Now().UTC().Format(time.RFC3339),
	})

	if logger := GetLogger(); logger != nil {
		logger.Warn("security", fmt.Sprintf("IP blocked globally: %s (reason: %s)", ip, reason))
	}

	return s.saveConfigLocked()
}

// UnblockIP removes an IP from the global blocklist.
func (s *SecurityService) UnblockIP(ip string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	found := false
	filtered := make([]BlockedIP, 0, len(s.config.BlockedIPs))
	for _, b := range s.config.BlockedIPs {
		if b.IP != ip {
			filtered = append(filtered, b)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("IP %s is not in the blocklist", ip)
	}

	s.config.BlockedIPs = filtered

	if logger := GetLogger(); logger != nil {
		logger.Info("security", fmt.Sprintf("IP unblocked: %s", ip))
	}

	return s.saveConfigLocked()
}

// CalculateScore computes a 0-100 security score for a domain config.
func (s *SecurityService) CalculateScore(cfg DomainSecurityConfig) int {
	score := 0
	maxScore := 0

	// HSTS (25 points)
	maxScore += 25
	if cfg.HSTSEnabled {
		score += 15
		if cfg.HSTSSubdomains {
			score += 5
		}
		if cfg.HSTSPreload {
			score += 5
		}
	}

	// X-Frame-Options (15 points)
	maxScore += 15
	if cfg.XFrameOptions == "DENY" {
		score += 15
	} else if cfg.XFrameOptions == "SAMEORIGIN" {
		score += 10
	}

	// X-Content-Type-Options (15 points)
	maxScore += 15
	if cfg.XContentTypeOpts {
		score += 15
	}

	// Referrer-Policy (15 points)
	maxScore += 15
	if cfg.ReferrerPolicy != "" {
		if cfg.ReferrerPolicy == "no-referrer" || cfg.ReferrerPolicy == "strict-origin" {
			score += 15
		} else {
			score += 10
		}
	}

	// Permissions-Policy (10 points)
	maxScore += 10
	if cfg.PermissionsPolicy != "" {
		score += 10
	}

	// Content-Security-Policy (20 points)
	maxScore += 20
	if cfg.CSPEnabled && cfg.CSPValue != "" {
		score += 20
	}

	if maxScore == 0 {
		return 0
	}
	return (score * 100) / maxScore
}

// applyDomainToCaddy pushes the security configuration to Caddy for a specific domain.
func (s *SecurityService) applyDomainToCaddy(cfg DomainSecurityConfig) error {
	if s.caddy == nil {
		return fmt.Errorf("caddy service not available")
	}

	// Find the route index for this domain
	domains, err := s.caddy.ListDomains()
	if err != nil {
		return fmt.Errorf("failed to list domains: %w", err)
	}

	routeIndex := -1
	for _, d := range domains {
		for _, dn := range d.Domains {
			if dn == cfg.Domain {
				routeIndex = d.ID
				break
			}
		}
		if routeIndex >= 0 {
			break
		}
	}

	if routeIndex < 0 {
		return fmt.Errorf("domain %s not found in Caddy config", cfg.Domain)
	}

	return s.caddy.ApplySecurityConfig(routeIndex, cfg)
}

// ApplyAllToCaddy re-applies all saved security configs to Caddy (used on startup).
func (s *SecurityService) ApplyAllToCaddy() {
	s.mu.RLock()
	configs := make([]DomainSecurityConfig, 0, len(s.config.Domains))
	for _, cfg := range s.config.Domains {
		configs = append(configs, cfg)
	}
	s.mu.RUnlock()

	applied := 0
	for _, cfg := range configs {
		if err := s.applyDomainToCaddy(cfg); err != nil {
			if logger := GetLogger(); logger != nil {
				logger.Warn("security", fmt.Sprintf("Could not re-apply security for %s: %v", cfg.Domain, err))
			}
		} else {
			applied++
		}
	}

	if logger := GetLogger(); logger != nil && applied > 0 {
		logger.Info("security", fmt.Sprintf("Re-applied security configs for %d domains on startup", applied))
	}
}
