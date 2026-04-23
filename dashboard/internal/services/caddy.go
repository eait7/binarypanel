package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	// First, ensure we have the base server config
	s.ensureServerExists()

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

	body, err := json.Marshal(route)
	if err != nil {
		return fmt.Errorf("failed to marshal route: %w", err)
	}

	resp, err := s.client.Post(
		s.apiURL+"/config/apps/http/servers/srv0/routes",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("failed to add site: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("caddy error (%d): %s", resp.StatusCode, string(respBody))
	}

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
