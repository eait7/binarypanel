package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Auth provides session-based authentication middleware.
type Auth struct {
	secret []byte
}

// NewAuth creates a new Auth middleware with the given signing secret.
func NewAuth(secret string) *Auth {
	return &Auth{secret: []byte(secret)}
}

// GenerateToken creates an HMAC-signed session token.
func (a *Auth) GenerateToken(username string) string {
	ts := fmt.Sprintf("%d", time.Now().Unix())
	payload := username + "|" + ts
	mac := hmac.New(sha256.New, a.secret)
	mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))
	return payload + "|" + sig
}

// ValidateToken verifies an HMAC-signed session token.
// Tokens expire after 24 hours.
func (a *Auth) ValidateToken(token string) (string, bool) {
	parts := strings.SplitN(token, "|", 3)
	if len(parts) != 3 {
		return "", false
	}
	username, tsStr, sig := parts[0], parts[1], parts[2]

	// Verify signature
	payload := username + "|" + tsStr
	mac := hmac.New(sha256.New, a.secret)
	mac.Write([]byte(payload))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return "", false
	}

	// Check expiry (24 hours)
	var ts int64
	fmt.Sscanf(tsStr, "%d", &ts)
	if time.Now().Unix()-ts > 86400 {
		return "", false
	}

	return username, true
}

// RequireAuth wraps an http.Handler and enforces authentication.
func (a *Auth) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("binarypanel_session")
		if err != nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		username, valid := a.ValidateToken(cookie.Value)
		if !valid {
			http.Error(w, `{"error":"session expired"}`, http.StatusUnauthorized)
			return
		}
		r.Header.Set("X-User", username)
		next.ServeHTTP(w, r)
	})
}
