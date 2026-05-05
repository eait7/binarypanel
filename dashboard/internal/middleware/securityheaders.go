package middleware

import (
	"net/http"
	"os"
)

// SecurityHeaders wraps a handler and injects hardened HTTP security headers on every response.
// HSTS is only emitted when BINARYPANEL_HTTPS=true (i.e., the panel is served over HTTPS).
func SecurityHeaders(next http.Handler) http.Handler {
	serveHTTPS := os.Getenv("BINARYPANEL_HTTPS") == "true"

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()

		// Prevent clickjacking.
		h.Set("X-Frame-Options", "DENY")

		// Prevent MIME-type sniffing.
		h.Set("X-Content-Type-Options", "nosniff")

		// Limit referrer information leakage.
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Restrict browser features.
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")

		// Content Security Policy: allow self + Google Fonts (used in the dashboard UI).
		h.Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self' 'unsafe-inline'; "+
				"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; "+
				"font-src 'self' https://fonts.gstatic.com; "+
				"img-src 'self' data:; "+
				"connect-src 'self'; "+
				"frame-ancestors 'none';",
		)

		// HSTS: only set over HTTPS to avoid breaking HTTP-only dev setups.
		if serveHTTPS {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		next.ServeHTTP(w, r)
	})
}
