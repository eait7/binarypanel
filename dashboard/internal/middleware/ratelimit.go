package middleware

import (
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// loginAttempt tracks failed login attempts for an IP.
type loginAttempt struct {
	count       int
	firstSeen   time.Time
	lockedUntil time.Time
}

// LoginRateLimiter provides brute-force protection for the login endpoint.
// It allows up to MaxAttempts per Window before locking out the IP for LockDuration.
type LoginRateLimiter struct {
	mu          sync.Mutex
	attempts    map[string]*loginAttempt
	MaxAttempts int
	Window      time.Duration
	LockDuration time.Duration
}

// NewLoginRateLimiter creates a new rate limiter and starts a background cleanup goroutine.
func NewLoginRateLimiter() *LoginRateLimiter {
	rl := &LoginRateLimiter{
		attempts:     make(map[string]*loginAttempt),
		MaxAttempts:  10,
		Window:       60 * time.Second,
		LockDuration: 15 * time.Minute,
	}
	// Purge stale entries every 5 minutes to prevent memory growth.
	go rl.cleanupLoop()
	return rl
}

// cleanupLoop removes expired entries from the attempts map.
func (rl *LoginRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, a := range rl.attempts {
			if now.After(a.lockedUntil) && now.Sub(a.firstSeen) > rl.Window {
				delete(rl.attempts, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// clientIP extracts the real client IP, respecting X-Forwarded-For from trusted proxies.
func clientIP(r *http.Request) string {
	// Use X-Forwarded-For only if set (we're behind Caddy).
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		// Take the first (leftmost) address — the original client.
		host, _, err := net.SplitHostPort(fwd)
		if err != nil {
			return fwd
		}
		return host
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// Allow checks whether the request is allowed and records the attempt.
// Returns (allowed bool, retryAfter seconds).
func (rl *LoginRateLimiter) Allow(r *http.Request) (bool, int) {
	ip := clientIP(r)
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	a, exists := rl.attempts[ip]

	if !exists {
		rl.attempts[ip] = &loginAttempt{count: 1, firstSeen: now, lockedUntil: time.Time{}}
		return true, 0
	}

	// If currently locked out, deny.
	if now.Before(a.lockedUntil) {
		retryAfter := int(a.lockedUntil.Sub(now).Seconds()) + 1
		return false, retryAfter
	}

	// Reset window if enough time has passed since first attempt.
	if now.Sub(a.firstSeen) > rl.Window {
		a.count = 1
		a.firstSeen = now
		a.lockedUntil = time.Time{}
		return true, 0
	}

	a.count++
	if a.count > rl.MaxAttempts {
		a.lockedUntil = now.Add(rl.LockDuration)
		retryAfter := int(rl.LockDuration.Seconds())
		return false, retryAfter
	}

	return true, 0
}

// RecordSuccess resets the attempt counter for an IP on a successful login.
func (rl *LoginRateLimiter) RecordSuccess(r *http.Request) {
	ip := clientIP(r)
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.attempts, ip)
}

// Middleware wraps a handler and enforces rate limiting.
func (rl *LoginRateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		allowed, retryAfter := rl.Allow(r)
		if !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"too many login attempts, please try again later"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}
