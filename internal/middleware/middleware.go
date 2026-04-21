package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type Middleware func(http.Handler) http.Handler

func Chain(handler http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		handler = mws[i](handler)
	}
	return handler
}

// Recovery catches panics and returns 500 instead of crashing the process.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rv := recover(); rv != nil {
				slog.Error("panic recovered", "error", rv, "path", r.URL.Path)
				http.Error(w, `{"error":"internal server error","code":"INTERNAL_ERROR"}`, http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type requestIDKey struct{}

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = generateID()
		}
		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), requestIDKey{}, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(sw, r)

		reqID, _ := r.Context().Value(requestIDKey{}).(string)
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", reqID,
		)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *statusWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.status = code
		w.wroteHeader = true
	}
	w.ResponseWriter.WriteHeader(code)
}

func Timeout(d time.Duration) Middleware {
	return func(next http.Handler) http.Handler {
		return http.TimeoutHandler(next, d, `{"error":"request timeout","code":"TIMEOUT"}`)
	}
}

// RateLimit returns a per-IP token-bucket rate limiter.
func RateLimit(rps float64) Middleware {
	var (
		mu       sync.Mutex
		visitors = make(map[string]*visitorEntry)
	)

	go cleanupVisitors(&mu, visitors, 1*time.Minute)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			limiter := getVisitor(&mu, visitors, ip, rps)

			if !limiter.Allow() {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "1")
				http.Error(w, `{"error":"rate limit exceeded","code":"RATE_LIMITED"}`, http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

type visitorEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func getVisitor(mu *sync.Mutex, visitors map[string]*visitorEntry, ip string, rps float64) *rate.Limiter {
	mu.Lock()
	defer mu.Unlock()

	v, ok := visitors[ip]
	if ok {
		v.lastSeen = time.Now()
		return v.limiter
	}

	limiter := rate.NewLimiter(rate.Limit(rps), int(rps)*2)
	visitors[ip] = &visitorEntry{limiter: limiter, lastSeen: time.Now()}
	return limiter
}

func cleanupVisitors(mu *sync.Mutex, visitors map[string]*visitorEntry, interval time.Duration) {
	for {
		time.Sleep(interval)
		mu.Lock()
		cutoff := time.Now().Add(-3 * interval)
		for ip, v := range visitors {
			if v.lastSeen.Before(cutoff) {
				delete(visitors, ip)
			}
		}
		mu.Unlock()
	}
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func generateID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
