package handlers

import (
	"net/http"
	"runtime"

	"go.novek.io/sales/internal/metrics"
	"go.novek.io/sales/internal/tracing"

	"github.com/gin-gonic/gin"
)

// MetricsHandler handles metrics-related HTTP requests
type MetricsHandler struct {
	metrics *metrics.Metrics
	tracer  tracing.Tracer
}

// NewMetricsHandler creates a new metrics handler
func NewMetricsHandler(metrics *metrics.Metrics, tracer tracing.Tracer) *MetricsHandler {
	return &MetricsHandler{
		metrics: metrics,
		tracer:  tracer,
	}
}

// HandleGetMetrics returns all metrics
func (h *MetricsHandler) HandleGetMetrics(c *gin.Context) {
	txn := h.tracer.StartTransaction("get-metrics")
	defer h.tracer.EndTransaction(txn)

	// Add some real-time system metrics
	h.metrics.SetGauge("goroutines", int64(runtime.NumGoroutine()))

	c.JSON(http.StatusOK, h.metrics.GetAllMetrics())
}

// HandleGetHealthCheck returns a simplified health status
func (h *MetricsHandler) HandleGetHealthCheck(c *gin.Context) {
	healthChecks := h.metrics.GetHealthChecks()

	// Calculate overall health
	healthy := true
	for _, status := range healthChecks {
		if !status {
			healthy = false
			break
		}
	}

	status := http.StatusOK
	if !healthy {
		status = http.StatusServiceUnavailable
	}

	c.JSON(status, gin.H{
		"status":  healthy,
		"details": healthChecks,
	})
}

// RegisterRoutes registers the handler's routes
func (h *MetricsHandler) RegisterRoutes(router *gin.Engine) {
	router.GET("/metrics", h.HandleGetMetrics)
	router.GET("/health", h.HandleGetHealthCheck)
}
