package api

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// Middleware contains HTTP middleware functions
type Middleware struct {
	logger *logrus.Logger
}

// NewMiddleware creates a new middleware instance
func NewMiddleware(logger *logrus.Logger) *Middleware {
	return &Middleware{
		logger: logger,
	}
}

// Logger logs HTTP requests
func (m *Middleware) Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Generate a request ID
		requestID := uuid.New().String()
		ctx := context.WithValue(r.Context(), "request_id", requestID)
		r = r.WithContext(ctx)

		// Set request ID header
		w.Header().Set("X-Request-ID", requestID)

		// Create a response wrapper to capture the status code
		wrapper := &responseWrapper{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// Call the next handler
		next.ServeHTTP(wrapper, r)

		// Log the request
		m.logger.WithFields(logrus.Fields{
			"method":      r.Method,
			"path":        r.URL.Path,
			"status":      wrapper.statusCode,
			"duration":    time.Since(start).String(),
			"request_id":  requestID,
			"remote_addr": r.RemoteAddr,
			"user_agent":  r.UserAgent(),
		}).Info("HTTP request")
	})
}

// Recover recovers from panics
func (m *Middleware) Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				requestID, _ := r.Context().Value("request_id").(string)
				m.logger.WithFields(logrus.Fields{
					"error":      err,
					"request_id": requestID,
				}).Error("Panic recovered")

				// Return a 500 error
				WriteError(w, ErrInternalServer)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// CORS adds CORS headers
func (m *Middleware) CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if the origin is allowed
			origin := r.Header.Get("Origin")
			allowedOrigin := ""

			// Allow all origins if the list contains "*"
			if len(allowedOrigins) == 1 && allowedOrigins[0] == "*" {
				allowedOrigin = "*"
			} else {
				// Check if the origin is in the allowed list
				for _, allowed := range allowedOrigins {
					if allowed == origin {
						allowedOrigin = origin
						break
					}
				}
			}

			// Set CORS headers
			if allowedOrigin != "" {
				w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Max-Age", "86400")
			}

			// Handle preflight requests
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}

			// Call the next handler
			next.ServeHTTP(w, r)
		})
	}
}

// responseWrapper wraps http.ResponseWriter to capture the status code
type responseWrapper struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code
func (w *responseWrapper) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}
