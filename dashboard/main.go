package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"binarypanel/internal/config"
	"binarypanel/internal/handlers"
	"binarypanel/internal/middleware"
	"binarypanel/internal/services"
)

func main() {
	cfg := config.Load()
	auth := middleware.NewAuth(cfg.Secret)

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

	// Initialize security service
	securitySvc := services.NewSecurityService("/data/security.json", caddySvc)

	// Re-apply saved security configs to Caddy on startup (async so boot isn't blocked)
	go securitySvc.ApplyAllToCaddy()

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(cfg, auth)
	domainsHandler := handlers.NewDomainsHandler(caddySvc, dockerSvc)
	systemHandler := handlers.NewSystemHandler(sysInfoSvc, cfg)
	dashboardHandler := handlers.NewDashboardHandler("/static")
	appsHandler := handlers.NewAppsHandler()
	securityHandler := handlers.NewSecurityHandler(securitySvc, caddySvc)
	var containersHandler *handlers.ContainersHandler
	if dockerSvc != nil {
		containersHandler = handlers.NewContainersHandler(dockerSvc)
	}

	mux := http.NewServeMux()

	// ── Auth endpoints (no auth required) ──
	mux.HandleFunc("/api/auth/login", authHandler.Login)
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

	// CORS + logging wrapper
	handler := loggingMiddleware(mux)

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("🚀 BinaryPanel Dashboard starting on %s", addr)
	log.Printf("   Caddy API: %s", cfg.CaddyAPI)
	log.Printf("   FileBrowser: %s", cfg.FileBrowserURL)
	log.Printf("   Portainer: %s", cfg.PortainerExternalURL)
	log.Fatal(http.ListenAndServe(addr, handler))
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			log.Printf("%s %s", r.Method, r.URL.Path)
		}
		next.ServeHTTP(w, r)
	})
}
