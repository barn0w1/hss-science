package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// IPRateLimiter implements per-IP token-bucket rate limiting with automatic
// eviction of inactive IP entries.
type IPRateLimiter struct {
	mu      sync.Mutex
	entries map[string]*ipEntry
	limit   rate.Limit
	burst   int
}

type ipEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewIPRateLimiter creates a limiter that allows burst requests per IP
// with a sustained rate of rps requests per second.
func NewIPRateLimiter(rps float64, burst int) *IPRateLimiter {
	return &IPRateLimiter{
		entries: make(map[string]*ipEntry),
		limit:   rate.Limit(rps),
		burst:   burst,
	}
}

func (l *IPRateLimiter) allow(r *http.Request) bool {
	if isInternalRequest(r) {
		return true
	}
	ip := clientIP(r)
	l.mu.Lock()
	e, ok := l.entries[ip]
	if !ok {
		e = &ipEntry{limiter: rate.NewLimiter(l.limit, l.burst)}
		l.entries[ip] = e
	}
	e.lastSeen = time.Now()
	limiter := e.limiter
	l.mu.Unlock()
	return limiter.Allow()
}

// Cleanup removes IP entries that have been idle for longer than ttl.
// Call periodically from a background goroutine.
func (l *IPRateLimiter) Cleanup(ttl time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	threshold := time.Now().Add(-ttl)
	for ip, e := range l.entries {
		if e.lastSeen.Before(threshold) {
			delete(l.entries, ip)
		}
	}
}

// Middleware returns an http.Handler middleware that enforces the rate limit,
// responding 429 when the limit is exceeded.
func (l *IPRateLimiter) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !l.allow(r) {
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// clientIP extracts the real client IP from the request.
// Priority: CF-Connecting-IP (Cloudflare Tunnel) > X-Forwarded-For (local proxy) > RemoteAddr.
func clientIP(r *http.Request) string {
	if cf := r.Header.Get("CF-Connecting-IP"); cf != "" {
		if ip := net.ParseIP(strings.TrimSpace(cf)); ip != nil {
			return ip.String()
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}

	if host == "127.0.0.1" || host == "::1" {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			idx := strings.IndexByte(xff, ',')
			candidate := xff
			if idx >= 0 {
				candidate = xff[:idx]
			}
			if ip := net.ParseIP(strings.TrimSpace(candidate)); ip != nil {
				return ip.String()
			}
		}
	}

	return host
}

// isInternalRequest returns true when the direct connection originates from
// a loopback or RFC-1918 private address (Kubernetes intra-cluster traffic,
// health checks, etc.).
func isInternalRequest(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || isPrivate(ip)
}

var privateRanges = []net.IPNet{
	{IP: net.IP{10, 0, 0, 0}, Mask: net.CIDRMask(8, 32)},
	{IP: net.IP{172, 16, 0, 0}, Mask: net.CIDRMask(12, 32)},
	{IP: net.IP{192, 168, 0, 0}, Mask: net.CIDRMask(16, 32)},
	// IPv6 ULA
	{IP: net.ParseIP("fc00::"), Mask: net.CIDRMask(7, 128)},
}

func isPrivate(ip net.IP) bool {
	for _, r := range privateRanges {
		if r.Contains(ip) {
			return true
		}
	}
	return false
}
