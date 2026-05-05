package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"binarypanel/internal/config"
	"binarypanel/internal/handlers"
	"binarypanel/internal/middleware"
	"binarypanel/internal/services"
)

func main() {
	cfg := config.Load()
	auth := middleware.NewAuth(cfg.Secret)
	rateLimiter := middleware.NewLoginRateLimiter()

	// Initialize settings
	config.InitSettings()

	// Initialize structured logger (must be before services that use it)
	logger := services.NewPanelLogger("/data/binarypanel.log", 200)
	logger.Info("system", "BinaryPanel dashboard starting up")

	// Initialize services
	caddySvc := services.NewCaddyService(cfg.CaddyAPI)
	sysInfoSvc := services.NewSysInfoService()
	dockerSvc, err := services.NewDockerService()
	if err != nil {
		log.Printf("WARNING: Docker service unavailable: %v", err)
		logger.Warn("docker", "Docker service unavailable: "+err.Error())
	}

	// Domain persistence store — saves domain configs to /data/domains.json
	// so they survive Caddy restarts without needing --resume.
	domainStore := services.NewDomainStore("/data")

	// Initialize security service
	securitySvc := services.NewSecurityService("/data/security.json", caddySvc)

	// On startup: re-register persisted domains and security configs in Caddy.
	// Done async (with delay) so Caddy has time to fully initialize first.
	go func() {
		time.Sleep(4 * time.Second)

		// Restore user-configured domains from /data/domains.json
		if saved, err := domainStore.Load(); err == nil && len(saved) > 0 {
			logger.Info("domains", fmt.Sprintf("Restoring %d domain(s) to Caddy...", len(saved)))
			for _, d := range saved {
				if err := caddySvc.AddSite(d.Domain, d.Upstream, d.Type); err != nil {
					logger.Warn("domains", fmt.Sprintf("Could not restore %s: %v", d.Domain, err))
				} else {
					logger.Info("domains", "Restored: "+d.Domain+" → "+d.Upstream)
				}
			}
		}

		// Re-apply security configs
		securitySvc.ApplyAllToCaddy()
	}()

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(cfg, auth)
	domainsHandler := handlers.NewDomainsHandler(caddySvc, dockerSvc, domainStore)
	systemHandler := handlers.NewSystemHandler(sysInfoSvc, cfg)
	dashboardHandler := handlers.NewDashboardHandler("/static")
	appsHandler := handlers.NewAppsHandler()
	securityHandler := handlers.NewSecurityHandler(securitySvc, caddySvc)
	var containersHandler *handlers.ContainersHandler
	if dockerSvc != nil {
		containersHandler = handlers.NewContainersHandler(dockerSvc)
	}

	mux := http.NewServeMux()

	// ── Auth endpoints (no auth required, but login is rate-limited) ──
	mux.Handle("/api/auth/login", rateLimiter.Middleware(http.HandlerFunc(authHandler.Login)))
	mux.HandleFunc("/api/auth/session", authHandler.Session)
	mux.HandleFunc("/api/auth/logout", authHandler.Logout)

	// ── Protected API endpoints ──
	protectedMux := http.NewServeMux()

	// Apps (1-Click Installer)
	protectedMux.HandleFunc("/api/apps/deploy/binarycms", appsHandler.DeployBinaryCMS)
	protectedMux.HandleFunc("/api/apps/deploy/searxng", appsHandler.DeploySearXNG)

	// System
	protectedMux.HandleFunc("/api/system/stats", systemHandler.Stats)
	protectedMux.HandleFunc("/api/links", systemHandler.Links)
	protectedMux.HandleFunc("/api/system/update", systemHandler.UpdateSystem)
	protectedMux.HandleFunc("/api/system/logs", systemHandler.Logs)
	protectedMux.HandleFunc("/api/system/reboot-stack", systemHandler.RebootStack)

	// Security
	protectedMux.HandleFunc("/api/security/domains", securityHandler.ListDomainSecurity)
	protectedMux.HandleFunc("/api/security/domain", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			securityHandler.GetDomainSecurity(w, r)
		case http.MethodPut:
			securityHandler.UpdateDomainSecurity(w, r)
		default:
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
	})
	protectedMux.HandleFunc("/api/security/ips", securityHandler.ListBlockedIPs)
	protectedMux.HandleFunc("/api/security/ips/block", securityHandler.BlockIP)
	protectedMux.HandleFunc("/api/security/ips/unblock", securityHandler.UnblockIP)

	// Settings
	protectedMux.HandleFunc("/api/settings/email", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handlers.GetEmailSettings(w, r)
		case http.MethodPut:
			handlers.UpdateEmailSettings(w, r)
		default:
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
	})
	protectedMux.HandleFunc("/api/settings/email/test", handlers.TestEmailSettings)
	protectedMux.HandleFunc("/api/settings/auth", authHandler.UpdateCredentials)

	// Domains
	protectedMux.HandleFunc("/api/domains", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			domainsHandler.List(w, r)
		case http.MethodPost:
			domainsHandler.Add(w, r)
		default:
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
	})

	protectedMux.HandleFunc("/api/domains/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path
		switch {
		case strings.HasSuffix(path, "/restore") && r.Method == http.MethodPost:
			domainsHandler.Restore(w, r)
		case strings.HasSuffix(path, "/restart") && r.Method == http.MethodPost:
			domainsHandler.Restart(w, r)
		case strings.HasSuffix(path, "/backup") && r.Method == http.MethodGet:
			domainsHandler.Backup(w, r)
		case r.Method == http.MethodDelete:
			domainsHandler.Delete(w, r)
		case r.Method == http.MethodPut:
			domainsHandler.Update(w, r)
		default:
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
	})

	// Caddy config
	protectedMux.HandleFunc("/api/caddy/config", domainsHandler.CaddyConfig)

	// Containers
	if containersHandler != nil {
		protectedMux.HandleFunc("/api/containers", containersHandler.List)
		protectedMux.HandleFunc("/api/containers/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			path := r.URL.Path
			switch {
			case strings.HasSuffix(path, "/start") && r.Method == http.MethodPost:
				containersHandler.Start(w, r)
			case strings.HasSuffix(path, "/stop") && r.Method == http.MethodPost:
				containersHandler.Stop(w, r)
			case strings.HasSuffix(path, "/restart") && r.Method == http.MethodPost:
				containersHandler.Restart(w, r)
			case strings.HasSuffix(path, "/logs") && r.Method == http.MethodGet:
				containersHandler.Logs(w, r)
			case r.Method == http.MethodDelete:
				containersHandler.Delete(w, r)
			default:
				http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			}
		})
	}

	// Mount protected routes with auth middleware
	mux.Handle("/api/", auth.RequireAuth(protectedMux))

	// ── Static files (SPA) ──
	mux.Handle("/", dashboardHandler)

	// ── Middleware chain (outermost to innermost): SecurityHeaders → CORS → Logging ──
	panelOrigin := os.Getenv("BINARYPANEL_ORIGIN") // e.g. "https://panel.example.com"
	handler := middleware.SecurityHeaders(
		corsMiddleware(panelOrigin,
			loggingMiddleware(mux),
		),
	)

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("🚀 BinaryPanel Dashboard starting on %s", addr)
	log.Printf("   Caddy API: %s", cfg.CaddyAPI)
	log.Printf("   FileBrowser: %s", cfg.FileBrowserURL)
	log.Printf("   Portainer: %s", cfg.PortainerExternalURL)

	// Auto-register the panel domain in Caddy with HTTPS if PANEL_DOMAIN is set.
	// Without PANEL_DOMAIN: panel is at https://ip:8443 via Caddy's internal TLS.
	// With PANEL_DOMAIN: panel upgrades to a real Let's Encrypt cert on port 443.
	if panelDomain := os.Getenv("PANEL_DOMAIN"); panelDomain != "" {
		caddySvc := services.NewCaddyService(cfg.CaddyAPI)
		go func() {
			// Wait for Caddy to be fully ready.
			time.Sleep(3 * time.Second)
			if err := caddySvc.EnsurePanelRoute(panelDomain, fmt.Sprintf("localhost:%s", cfg.Port)); err != nil {
				log.Printf("⚠️  Could not register panel domain %s in Caddy: %v", panelDomain, err)
				log.Printf("   Falling back to default: https://SERVER-IP:8443")
			} else {
				log.Printf("✅ BinaryPanel accessible at https://%s (Let's Encrypt SSL)", panelDomain)
			}
		}()
	} else {
		log.Printf("ℹ️  Panel URL: https://SERVER-IP:8443 (secure, internal TLS — accept browser warning)")
		log.Printf("   Tip: Set PANEL_DOMAIN=panel.yourdomain.com in .env for a proper SSL certificate")
	}

	log.Fatal(http.ListenAndServe(addr, handler))
}

// loggingMiddleware logs API requests only (not static assets).
// Sensitive paths like /api/auth/login are noted but credentials are never logged.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			log.Printf("%s %s", r.Method, r.URL.Path)
		}
		next.ServeHTTP(w, r)
	})
}

// corsMiddleware enforces same-origin CORS policy.
// If origin is empty the panel allows requests from any origin on the same host
// (suitable for direct IP access). Set BINARYPANEL_ORIGIN to lock it to a domain.
func corsMiddleware(allowedOrigin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			if allowedOrigin != "" {
				// Strict mode: only allow the configured origin.
				if origin == allowedOrigin {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Vary", "Origin")
				}
			}
			// No wildcard * — unlisted origins receive no CORS headers (browser blocks them).
		}

		// Handle pre-flight requests.
		if r.Method == http.MethodOptions {
			if allowedOrigin != "" && origin == allowedOrigin {
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
				w.Header().Set("Access-Control-Max-Age", "86400")
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
