package api

import (
	"encoding/json"
	"net/http"
	"runtime"

	"github.com/sirupsen/logrus"

	"go.novek.io/truck/internal/metrics"
)

// MetricsHandler handles requests to get metrics
func MetricsHandler(w http.ResponseWriter, r *http.Request) {
	collector := metrics.GetMetricsCollector()

	// Get memory stats
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Update memory gauge
	collector.SetGauge(metrics.GaugeSystemMemory, float64(memStats.Alloc))

	// Get metrics
	metricData := collector.GetMetrics()

	// Add runtime info
	metricData["runtime"] = map[string]interface{}{
		"goroutines": runtime.NumGoroutine(),
		"memory": map[string]interface{}{
			"alloc_bytes":       memStats.Alloc,
			"total_alloc_bytes": memStats.TotalAlloc,
			"sys_bytes":         memStats.Sys,
			"heap_objects":      memStats.HeapObjects,
			"gc_cycles":         memStats.NumGC,
		},
	}

	// Write response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(metricData); err != nil {
		logrus.WithError(err).Error("Failed to encode metrics response")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// HealthHandler handles health check requests
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	collector := metrics.GetMetricsCollector()
	health := collector.GetHealthStatus()

	// Determine HTTP status code based on health
	statusCode := http.StatusOK
	if healthStatus, ok := health["status"].(map[string]interface{}); ok {
		if healthy, ok := healthStatus["healthy"].(bool); ok && !healthy {
			statusCode = http.StatusServiceUnavailable
		}
	}

	// Write response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(health); err != nil {
		logrus.WithError(err).Error("Failed to encode health response")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
