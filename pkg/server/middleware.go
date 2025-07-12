package server

import (
	"context"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// Middleware represents HTTP middleware
type Middleware func(http.Handler) http.Handler

// LoggingMiddleware adds request logging
func LoggingMiddleware(logger *zap.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Create a response writer wrapper to capture status code
			rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			// Log request
			logger.Info("Request received",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.String("remote_addr", r.RemoteAddr),
				zap.String("user_agent", r.UserAgent()),
				zap.String("session_id", r.Header.Get("Mcp-Session-Id")))

			next.ServeHTTP(rw, r)

			// Log response
			logger.Info("Request completed",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", rw.statusCode),
				zap.Duration("duration", time.Since(start)))
		})
	}
}

// CORSMiddleware adds CORS headers
func CORSMiddleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Mcp-Session-Id")
			w.Header().Set("Access-Control-Expose-Headers", "Mcp-Session-Id")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// SecurityMiddleware adds security headers
func SecurityMiddleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Security headers
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

			// Content Security Policy
			csp := "default-src 'self'; " +
				"script-src 'self' 'unsafe-inline'; " +
				"style-src 'self' 'unsafe-inline'; " +
				"img-src 'self' data: https:; " +
				"connect-src 'self'"
			w.Header().Set("Content-Security-Policy", csp)

			next.ServeHTTP(w, r)
		})
	}
}

// RateLimitMiddleware adds rate limiting
func RateLimitMiddleware(requestsPerSecond int, burst int) Middleware {
	limiter := rate.NewLimiter(rate.Limit(requestsPerSecond), burst)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !limiter.Allow() {
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// SessionRateLimitMiddleware adds per-session rate limiting
func SessionRateLimitMiddleware(requestsPerSecond int, burst int) Middleware {
	limiters := make(map[string]*rate.Limiter)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sessionID := r.Header.Get("Mcp-Session-Id")
			if sessionID == "" {
				sessionID = "anonymous"
			}

			// Get or create limiter for this session
			limiter, exists := limiters[sessionID]
			if !exists {
				limiter = rate.NewLimiter(rate.Limit(requestsPerSecond), burst)
				limiters[sessionID] = limiter
			}

			if !limiter.Allow() {
				http.Error(w, "Rate limit exceeded for session", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ContentTypeMiddleware validates content type
func ContentTypeMiddleware(allowedTypes ...string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "POST" || r.Method == "PUT" {
				contentType := r.Header.Get("Content-Type")
				if contentType == "" {
					http.Error(w, "Content-Type header is required", http.StatusBadRequest)
					return
				}

				// Check if content type is allowed
				allowed := false
				for _, allowedType := range allowedTypes {
					if strings.Contains(contentType, allowedType) {
						allowed = true
						break
					}
				}

				if !allowed {
					http.Error(w, "Unsupported content type", http.StatusUnsupportedMediaType)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequestSizeMiddleware limits request body size
func RequestSizeMiddleware(maxBytes int64) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > maxBytes {
				http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
				return
			}

			// Limit the request body reader
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)

			next.ServeHTTP(w, r)
		})
	}
}

// TimeoutMiddleware adds request timeout
func TimeoutMiddleware(timeout time.Duration) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()

			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)
		})
	}
}

// RecoveryMiddleware recovers from panics
func RecoveryMiddleware(logger *zap.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					logger.Error("Panic recovered",
						zap.String("method", r.Method),
						zap.String("path", r.URL.Path),
						zap.Any("error", err))

					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// AuthMiddleware adds authentication (placeholder for future implementation)
func AuthMiddleware(authFunc func(string) bool) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip authentication for health check endpoints
			if r.URL.Path == "/health" || r.URL.Path == "/metrics" {
				next.ServeHTTP(w, r)
				return
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				// For now, allow requests without authentication
				// In production, this should be uncommented:
				// http.Error(w, "Authorization header required", http.StatusUnauthorized)
				// return
			} else {
				// Validate token
				token := strings.TrimPrefix(authHeader, "Bearer ")
				if !authFunc(token) {
					http.Error(w, "Invalid token", http.StatusUnauthorized)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// MetricsMiddleware adds metrics collection (placeholder)
func MetricsMiddleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(rw, r)

			// Here you would record metrics like:
			// - Request duration
			// - Status code
			// - Request count
			// - Error count

			duration := time.Since(start)
			_ = duration // Prevent unused variable warning
			_ = rw.statusCode
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// ChainMiddleware chains multiple middleware functions
func ChainMiddleware(middlewares ...Middleware) Middleware {
	return func(next http.Handler) http.Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
}

// ValidateJSONRPC validates JSON-RPC requests
func ValidateJSONRPC() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" {
				next.ServeHTTP(w, r)
				return
			}

			// Only validate JSON-RPC for POST requests to the main endpoint
			if r.URL.Path != "/" {
				next.ServeHTTP(w, r)
				return
			}

			// The actual JSON-RPC validation will be done in the handler
			// This middleware can be extended to do basic validation

			next.ServeHTTP(w, r)
		})
	}
}

// DefaultMiddleware returns a set of default middleware
func DefaultMiddleware(logger *zap.Logger) []Middleware {
	return []Middleware{
		RecoveryMiddleware(logger),
		LoggingMiddleware(logger),
		SecurityMiddleware(),
		CORSMiddleware(),
		RateLimitMiddleware(100, 200), // 100 requests per second, burst of 200
		ContentTypeMiddleware("application/json"),
		RequestSizeMiddleware(1024 * 1024),  // 1MB max request size
		TimeoutMiddleware(30 * time.Second), // 30 second timeout
		MetricsMiddleware(),
		ValidateJSONRPC(),
	}
}
