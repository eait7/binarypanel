package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// CaddyService provides an interface to Caddy's Admin API.
type CaddyService struct {
	apiURL string
	client *http.Client
}

// DomainInfo represents a configured domain/site.
type DomainInfo struct {
	ID       int      `json:"id"`
	Domains  []string `json:"domains"`
	Upstream string   `json:"upstream"`
	Type     string   `json:"type"` // "reverse_proxy" or "file_server"
	TLS      bool     `json:"tls"`
	// Security status fields
	SecurityScore  int  `json:"security_score"`
	HeadersEnabled bool `json:"headers_enabled"`
	IPRestricted   bool `json:"ip_restricted"`
}

// CaddyRoute represents a route in Caddy's JSON config.
type CaddyRoute struct {
	Match  []map[string]interface{} `json:"match,omitempty"`
	Handle []map[string]interface{} `json:"handle,omitempty"`
}

// NewCaddyService creates a new Caddy API client.
func NewCaddyService(apiURL string) *CaddyService {
	return &CaddyService{
		apiURL: apiURL,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// GetConfig fetches the full Caddy JSON configuration.
func (s *CaddyService) GetConfig() (map[string]interface{}, error) {
	resp, err := s.client.Get(s.apiURL + "/config/")
	if err != nil {
		if logger := GetLogger(); logger != nil {
			diag := DiagnoseCaddyError(err)
			logger.Error("caddy", "Failed to fetch Caddy config: "+err.Error(), diag)
		}
		return nil, fmt.Errorf("caddy api unreachable: %w", err)
	}
	defer resp.Body.Close()

	var config map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to decode caddy config: %w", err)
	}
	return config, nil
}

// ListDomains parses the current Caddy config and returns a list of configured domains.
func (s *CaddyService) ListDomains() ([]DomainInfo, error) {
	resp, err := s.client.Get(s.apiURL + "/config/apps/http/servers")
	if err != nil {
		if logger := GetLogger(); logger != nil {
			diag := DiagnoseCaddyError(err)
			logger.Error("caddy", "Failed to list domains: "+err.Error(), diag)
		}
		return nil, fmt.Errorf("caddy api unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		// No servers configured yet
		return []DomainInfo{}, nil
	}

	body, _ := io.ReadAll(resp.Body)

	var servers map[string]interface{}
	if err := json.Unmarshal(body, &servers); err != nil {
		return []DomainInfo{}, nil
	}

	var domains []DomainInfo
	routeIdx := 0

	for _, serverVal := range servers {
		server, ok := serverVal.(map[string]interface{})
		if !ok {
			continue
		}
		routes, ok := server["routes"].([]interface{})
		if !ok {
			continue
		}
		for _, routeVal := range routes {
			route, ok := routeVal.(map[string]interface{})
			if !ok {
				continue
			}
			info := DomainInfo{ID: routeIdx, TLS: true}
			routeIdx++

			// Extract host matchers
			if matches, ok := route["match"].([]interface{}); ok {
				for _, matchVal := range matches {
					match, ok := matchVal.(map[string]interface{})
					if !ok {
						continue
					}
					if hosts, ok := match["host"].([]interface{}); ok {
						for _, h := range hosts {
							if host, ok := h.(string); ok {
								info.Domains = append(info.Domains, host)
							}
						}
					}
				}
			}

			// Extract handler type and upstream
			if handles, ok := route["handle"].([]interface{}); ok {
				for _, handleVal := range handles {
					handle, ok := handleVal.(map[string]interface{})
					if !ok {
						continue
					}
					handler, _ := handle["handler"].(string)
					if handler == "reverse_proxy" {
						info.Type = "reverse_proxy"
						if upstreams, ok := handle["upstreams"].([]interface{}); ok && len(upstreams) > 0 {
							if us, ok := upstreams[0].(map[string]interface{}); ok {
								info.Upstream, _ = us["dial"].(string)
							}
						}
					} else if handler == "file_server" {
						info.Type = "file_server"
						info.Upstream, _ = handle["root"].(string)
					} else if handler == "subroute" {
						// Handle subroute wrapper
						if subRoutes, ok := handle["routes"].([]interface{}); ok {
							for _, sr := range subRoutes {
								if srMap, ok := sr.(map[string]interface{}); ok {
									if subHandles, ok := srMap["handle"].([]interface{}); ok {
										for _, sh := range subHandles {
											if shMap, ok := sh.(map[string]interface{}); ok {
												h, _ := shMap["handler"].(string)
												if h == "reverse_proxy" {
													info.Type = "reverse_proxy"
													if ups, ok := shMap["upstreams"].([]interface{}); ok && len(ups) > 0 {
														if u, ok := ups[0].(map[string]interface{}); ok {
															info.Upstream, _ = u["dial"].(string)
														}
													}
												} else if h == "file_server" {
													info.Type = "file_server"
													info.Upstream, _ = shMap["root"].(string)
												}
											}
										}
									}
								}
							}
						}
					}
				}
			}

			if len(info.Domains) > 0 {
				domains = append(domains, info)
			}
		}
	}
	return domains, nil
}

// AddSite adds a new domain/site to Caddy's configuration.
func (s *CaddyService) AddSite(domain, upstream, handlerType string) error {
	// Translate 'localhost' to the Docker bridge gateway so Caddy (inside its
	// own container) can reach services running on the host or other containers
	// that expose ports on the host. The binarypanel network gateway is 172.28.0.1.
	upstream = strings.ReplaceAll(upstream, "localhost:", "172.28.0.1:")
	upstream = strings.ReplaceAll(upstream, "127.0.0.1:", "172.28.0.1:")

	var handler map[string]interface{}
	if handlerType == "file_server" {
		handler = map[string]interface{}{
			"handler": "file_server",
			"root":    upstream,
		}
	} else {
		handler = map[string]interface{}{
			"handler": "reverse_proxy",
			"upstreams": []map[string]interface{}{
				{"dial": upstream},
			},
		}
	}

	route := map[string]interface{}{
		"match": []map[string]interface{}{
			{"host": []string{domain}},
		},
		"handle": []map[string]interface{}{
			{
				"handler": "subroute",
				"routes": []map[string]interface{}{
					{
						"handle": []interface{}{handler},
					},
				},
			},
		},
		"terminal": true,
	}

	// Add to HTTPS server (srv_domains on :443) — Let's Encrypt handles certs.
	if err := s.ensureDomainServerExists(); err == nil {
		s.prependRoute("srv_domains", route)
	}

	// Prepend to srv0 so domain routes fire BEFORE the catch-all default on port 80.
	body, err := json.Marshal(route)
	if err != nil {
		return fmt.Errorf("failed to marshal route: %w", err)
	}

	if err := s.prependRoute("srv0", route); err != nil {
		// Fallback: append
		resp, err2 := s.client.Post(
			s.apiURL+"/config/apps/http/servers/srv0/routes",
			"application/json",
			bytes.NewReader(body),
		)
		if err2 != nil {
			return fmt.Errorf("failed to add site: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			respBody, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("caddy error (%d): %s", resp.StatusCode, string(respBody))
		}
	}

	return nil
}

// prependRoute inserts a route at position 0 of the named server's routes list,
// ensuring it fires before any catch-all default routes.
func (s *CaddyService) prependRoute(serverName string, route map[string]interface{}) error {
	// GET current routes
	resp, err := s.client.Get(fmt.Sprintf("%s/config/apps/http/servers/%s/routes", s.apiURL, serverName))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var routes []interface{}
	if err := json.NewDecoder(resp.Body).Decode(&routes); err != nil {
		routes = []interface{}{}
	}

	// Prepend our route
	routes = append([]interface{}{route}, routes...)

	body, err := json.Marshal(routes)
	if err != nil {
		return err
	}

	// PATCH full routes array back
	req, err := http.NewRequest("PATCH",
		fmt.Sprintf("%s/config/apps/http/servers/%s/routes", s.apiURL, serverName),
		bytes.NewReader(body),
	)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp2, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp2.Body.Close()
	if resp2.StatusCode >= 400 {
		body2, _ := io.ReadAll(resp2.Body)
		return fmt.Errorf("caddy PATCH routes error (%d): %s", resp2.StatusCode, string(body2))
	}
	return nil
}

// ensureDomainServerExists creates the srv_domains server (port 443) if missing.
// This server is where user domains with Let's Encrypt certs live.
func (s *CaddyService) ensureDomainServerExists() error {
	resp, err := s.client.Get(s.apiURL + "/config/apps/http/servers/srv_domains")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 404 {
		return nil // already exists
	}
	server := map[string]interface{}{
		"listen": []string{":443"},
		"routes": []interface{}{},
	}
	body, _ := json.Marshal(server)
	req, _ := http.NewRequest("PUT",
		s.apiURL+"/config/apps/http/servers/srv_domains",
		bytes.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")
	resp2, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp2.Body.Close()
	return nil
}

// RemoveSite removes a domain/site from Caddy's configuration by route index.
func (s *CaddyService) RemoveSite(routeIndex int) error {
	req, err := http.NewRequest(
		"DELETE",
		fmt.Sprintf("%s/config/apps/http/servers/srv0/routes/%d", s.apiURL, routeIndex),
		nil,
	)
	if err != nil {
		return err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		if logger := GetLogger(); logger != nil {
			diag := DiagnoseCaddyError(err)
			logger.Error("caddy", fmt.Sprintf("Failed to remove site at index %d: %v", routeIndex, err), diag)
		}
		return fmt.Errorf("failed to remove site: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("caddy error (%d): %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// UpdateSite updates a domain/site configuration.
func (s *CaddyService) UpdateSite(routeIndex int, domain, upstream, handlerType string) error {
	// Remove then re-add (simplest approach for route updates)
	if err := s.RemoveSite(routeIndex); err != nil {
		return fmt.Errorf("failed to remove old route: %w", err)
	}
	return s.AddSite(domain, upstream, handlerType)
}

// ensureServerExists creates the base HTTP server in Caddy if it doesn't exist.
func (s *CaddyService) ensureServerExists() {
	// Check if srv0 exists
	resp, err := s.client.Get(s.apiURL + "/config/apps/http/servers/srv0")
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		// Create the server
		server := map[string]interface{}{
			"listen": []string{":443", ":80"},
			"routes": []interface{}{},
		}
		body, _ := json.Marshal(server)

		req, _ := http.NewRequest("PUT", s.apiURL+"/config/apps/http/servers/srv0", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r, err := s.client.Do(req)
		if err == nil {
			r.Body.Close()
		}
	}
}

// ReloadConfig sends a full config reload to Caddy.
func (s *CaddyService) ReloadConfig(config map[string]interface{}) error {
	body, err := json.Marshal(config)
	if err != nil {
		return err
	}

	resp, err := s.client.Post(
		s.apiURL+"/load",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("failed to reload caddy: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("caddy reload error (%d): %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// ApplySecurityConfig applies security headers and IP restrictions to a Caddy route.
func (s *CaddyService) ApplySecurityConfig(routeIndex int, cfg DomainSecurityConfig) error {
	// First, get the current route to rebuild its subroute handlers
	resp, err := s.client.Get(fmt.Sprintf("%s/config/apps/http/servers/srv0/routes/%d/handle/0/routes", s.apiURL, routeIndex))
	if err != nil {
		return fmt.Errorf("failed to read route: %w", err)
	}
	defer resp.Body.Close()

	var existingRoutes []map[string]interface{}
	if resp.StatusCode == 200 {
		body, _ := io.ReadAll(resp.Body)
		json.Unmarshal(body, &existingRoutes)
	}

	// Filter out any existing security-related routes (headers handler and IP block responder)
	var cleanRoutes []map[string]interface{}
	for _, route := range existingRoutes {
		if handles, ok := route["handle"].([]interface{}); ok {
			keep := true
			for _, h := range handles {
				if hm, ok := h.(map[string]interface{}); ok {
					handler, _ := hm["handler"].(string)
					if handler == "headers" {
						// Check if this is our security headers handler (has marker)
						if respH, ok := hm["response"].(map[string]interface{}); ok {
							if set, ok := respH["set"].(map[string]interface{}); ok {
								if _, ok := set["X-Binarypanel-Security"]; ok {
									keep = false
								}
							}
						}
					} else if handler == "static_response" {
						// IP block response handler
						if code, ok := hm["status_code"].(string); ok && code == "403" {
							keep = false
						}
						if code, ok := hm["status_code"].(float64); ok && int(code) == 403 {
							keep = false
						}
					}
				}
			}
			if !keep {
				continue
			}
		}
		cleanRoutes = append(cleanRoutes, route)
	}

	// Build new security routes to prepend
	var securityRoutes []map[string]interface{}

	// 1. IP Blacklist route (respond 403 for blocked IPs)
	if len(cfg.IPBlacklist) > 0 {
		ipBlockRoute := map[string]interface{}{
			"match": []map[string]interface{}{
				{"remote_ip": map[string]interface{}{"ranges": cfg.IPBlacklist}},
			},
			"handle": []map[string]interface{}{
				{
					"handler":     "static_response",
					"status_code": "403",
					"body":        "Access denied by BinaryPanel security policy.",
				},
			},
			"terminal": true,
		}
		securityRoutes = append(securityRoutes, ipBlockRoute)
	}

	// 2. IP Whitelist route (respond 403 for anything NOT in the whitelist)
	if len(cfg.IPWhitelist) > 0 {
		ipWhitelistRoute := map[string]interface{}{
			"match": []map[string]interface{}{
				{
					"not": []map[string]interface{}{
						{"remote_ip": map[string]interface{}{"ranges": cfg.IPWhitelist}},
					},
				},
			},
			"handle": []map[string]interface{}{
				{
					"handler":     "static_response",
					"status_code": "403",
					"body":        "Access denied by BinaryPanel security policy.",
				},
			},
			"terminal": true,
		}
		securityRoutes = append(securityRoutes, ipWhitelistRoute)
	}

	// 3. Security headers route
	headerSet := make(map[string][]string)

	// Marker header so we can identify our injected headers later
	headerSet["X-Binarypanel-Security"] = []string{"active"}

	if cfg.HSTSEnabled {
		hstsValue := fmt.Sprintf("max-age=%d", cfg.HSTSMaxAge)
		if cfg.HSTSSubdomains {
			hstsValue += "; includeSubDomains"
		}
		if cfg.HSTSPreload {
			hstsValue += "; preload"
		}
		headerSet["Strict-Transport-Security"] = []string{hstsValue}
	}

	if cfg.XFrameOptions != "" {
		headerSet["X-Frame-Options"] = []string{cfg.XFrameOptions}
	}

	if cfg.XContentTypeOpts {
		headerSet["X-Content-Type-Options"] = []string{"nosniff"}
	}

	if cfg.ReferrerPolicy != "" {
		headerSet["Referrer-Policy"] = []string{cfg.ReferrerPolicy}
	}

	if cfg.PermissionsPolicy != "" {
		headerSet["Permissions-Policy"] = []string{cfg.PermissionsPolicy}
	}

	if cfg.CSPEnabled && cfg.CSPValue != "" {
		headerSet["Content-Security-Policy"] = []string{cfg.CSPValue}
	}

	// Only add headers route if there's something beyond the marker
	if len(headerSet) > 1 {
		headersRoute := map[string]interface{}{
			"handle": []map[string]interface{}{
				{
					"handler": "headers",
					"response": map[string]interface{}{
						"set": headerSet,
					},
				},
			},
		}
		securityRoutes = append(securityRoutes, headersRoute)
	}

	// Combine: security routes first, then existing clean routes
	allRoutes := append(securityRoutes, cleanRoutes...)

	// Push the rebuilt subroute back to Caddy
	routeBody, err := json.Marshal(allRoutes)
	if err != nil {
		return fmt.Errorf("failed to marshal routes: %w", err)
	}

	patchURL := fmt.Sprintf("%s/config/apps/http/servers/srv0/routes/%d/handle/0/routes", s.apiURL, routeIndex)
	req, err := http.NewRequest("PATCH", patchURL, bytes.NewReader(routeBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	patchResp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to patch caddy route: %w", err)
	}
	defer patchResp.Body.Close()

	if patchResp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(patchResp.Body)
		return fmt.Errorf("caddy rejected security config (HTTP %d): %s", patchResp.StatusCode, string(respBody))
	}

	return nil
}

// EnsurePanelRoute idempotently registers BinaryPanel's own subdomain in Caddy.
// Called on startup when PANEL_DOMAIN env var is set. Safe to call multiple times.
func (s *CaddyService) EnsurePanelRoute(domain, upstream string) error {
	// Check if the domain is already configured so we don't duplicate it.
	domains, err := s.ListDomains()
	if err != nil {
		// Caddy might still be starting; treat as non-fatal.
		return fmt.Errorf("could not list domains: %w", err)
	}
	for _, d := range domains {
		for _, h := range d.Domains {
			if h == domain {
				// Already registered — nothing to do.
				return nil
			}
		}
	}
	// Register the panel domain as a reverse-proxy route with TLS (Caddy handles cert).
	return s.AddSite(domain, upstream, "reverse_proxy")
}
