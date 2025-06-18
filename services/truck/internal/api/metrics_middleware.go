package api

import (
	"net/http"
	"time"

	"go.novek.io/truck/internal/metrics"
)

// MetricsMiddleware adds metrics collection to HTTP requests
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a response wrapper to capture the status code
		wrapper := &responseWrapper{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// Call the next handler
		next.ServeHTTP(wrapper, r)

		// Record metrics
		duration := time.Since(start)
		collector := metrics.GetMetricsCollector()
		collector.RecordHTTPRequest(r.URL.Path, wrapper.statusCode, duration)
	})
}
