package metrics

import (
	"sync"
	"time"
)

// MetricsCollector provides a centralized way to collect and retrieve metrics
type MetricsCollector struct {
	mutex               sync.RWMutex
	counters            map[string]int64
	gauges              map[string]float64
	histograms          map[string][]float64
	requestLatencies    map[string][]time.Duration
	requestCounts       map[string]int64
	operationCounts     map[string]int64
	operationLatencies  map[string][]time.Duration
	messageBusCounts    map[string]int64
	messageBusLatencies map[string][]time.Duration
	databaseQueryCounts map[string]int64
	databaseLatencies   map[string][]time.Duration
	errorCounts         map[string]int64
	startTime           time.Time
	maxHistogramSamples int
}

// Counter metrics
const (
	CounterHTTPRequests        = "http_requests_total"
	CounterHTTPRequestsSuccess = "http_requests_success_total"
	CounterHTTPRequestsError   = "http_requests_error_total"
	CounterOperationsCreated   = "operations_created_total"
	CounterOperationsCompleted = "operations_completed_total"
	CounterOperationsFailed    = "operations_failed_total"
	CounterMessagesSent        = "messages_sent_total"
	CounterMessagesReceived    = "messages_received_total"
	CounterMessagesProcessed   = "messages_processed_total"
	CounterMessagesError       = "messages_error_total"
	CounterDBQueriesTotal      = "db_queries_total"
	CounterDBQueriesError      = "db_queries_error_total"
	CounterErrorsTotal         = "errors_total"
)

// Gauge metrics
const (
	GaugeActiveOperations      = "active_operations"
	GaugeActiveOperationGroups = "active_operation_groups"
	GaugePendingMessages       = "pending_messages"
	GaugeSystemMemory          = "system_memory_bytes"
	GaugeCPUUsage              = "cpu_usage_percent"
)

// Operation types for operation metrics
const (
	OperationTypeCreate          = "create"
	OperationTypeUpdate          = "update"
	OperationTypeComplete        = "complete"
	OperationTypeFailed          = "failed"
	OperationTypeCancel          = "cancel"
	OperationTypeEventProcessing = "event_processing"
)

// Database query types
const (
	DBQueryTypeSelect = "select"
	DBQueryTypeInsert = "insert"
	DBQueryTypeUpdate = "update"
	DBQueryTypeDelete = "delete"
)

// Message bus operations
const (
	MessageBusOperationSend     = "send"
	MessageBusOperationReceive  = "receive"
	MessageBusOperationComplete = "complete"
	MessageBusOperationReject   = "reject"
)

// Error types
const (
	ErrorTypeHTTP       = "http"
	ErrorTypeValidation = "validation"
	ErrorTypeDatabase   = "database"
	ErrorTypeMessageBus = "message_bus"
	ErrorTypeInternal   = "internal"
)

// HTTP paths
const (
	HTTPPathActiveOperation      = "/ops/op/{device_uid}"
	HTTPPathOperationEvent       = "/ops/op/{device_uid}/events"
	HTTPPathActiveOperationGroup = "/ops/opg/{truck_uid}"
	HTTPPathOperationGroupEvent  = "/ops/opg/{truck_uid}/events"
	HTTPPathMetrics              = "/metrics"
	HTTPPathHealth               = "/health"
)

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		counters:            make(map[string]int64),
		gauges:              make(map[string]float64),
		histograms:          make(map[string][]float64),
		requestLatencies:    make(map[string][]time.Duration),
		requestCounts:       make(map[string]int64),
		operationCounts:     make(map[string]int64),
		operationLatencies:  make(map[string][]time.Duration),
		messageBusCounts:    make(map[string]int64),
		messageBusLatencies: make(map[string][]time.Duration),
		databaseQueryCounts: make(map[string]int64),
		databaseLatencies:   make(map[string][]time.Duration),
		errorCounts:         make(map[string]int64),
		startTime:           time.Now(),
		maxHistogramSamples: 1000,
	}
}

// IncrementCounter increments a counter by the given value
func (m *MetricsCollector) IncrementCounter(name string, value int64) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.counters[name] += value
}

// SetGauge sets a gauge to the given value
func (m *MetricsCollector) SetGauge(name string, value float64) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.gauges[name] = value
}

// RecordLatency records a latency value for histogram creation
func (m *MetricsCollector) RecordLatency(name string, value time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	latencies, ok := m.histograms[name]
	if !ok {
		latencies = make([]float64, 0, m.maxHistogramSamples)
	}
	if len(latencies) >= m.maxHistogramSamples {
		// Remove the oldest sample
		latencies = latencies[1:]
	}
	latencies = append(latencies, float64(value.Milliseconds()))
	m.histograms[name] = latencies
}

// RecordHTTPRequest records metrics for an HTTP request
func (m *MetricsCollector) RecordHTTPRequest(path string, statusCode int, latency time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Increment total request counter
	m.counters[CounterHTTPRequests]++

	// Record request by path
	m.requestCounts[path]++

	// Record latency
	latencies, ok := m.requestLatencies[path]
	if !ok {
		latencies = make([]time.Duration, 0, m.maxHistogramSamples)
	}
	if len(latencies) >= m.maxHistogramSamples {
		latencies = latencies[1:]
	}
	latencies = append(latencies, latency)
	m.requestLatencies[path] = latencies

	// Count successful/error requests
	if statusCode >= 200 && statusCode < 400 {
		m.counters[CounterHTTPRequestsSuccess]++
	} else {
		m.counters[CounterHTTPRequestsError]++
		m.errorCounts[ErrorTypeHTTP]++
	}
}

// RecordOperation records metrics for an operation
func (m *MetricsCollector) RecordOperation(operationType string, latency time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Increment operation counter
	m.operationCounts[operationType]++

	// Record operation-specific counters
	switch operationType {
	case OperationTypeCreate:
		m.counters[CounterOperationsCreated]++
	case OperationTypeComplete:
		m.counters[CounterOperationsCompleted]++
	case OperationTypeFailed:
		m.counters[CounterOperationsFailed]++
		m.errorCounts[ErrorTypeInternal]++
	}

	// Record latency
	latencies, ok := m.operationLatencies[operationType]
	if !ok {
		latencies = make([]time.Duration, 0, m.maxHistogramSamples)
	}
	if len(latencies) >= m.maxHistogramSamples {
		latencies = latencies[1:]
	}
	latencies = append(latencies, latency)
	m.operationLatencies[operationType] = latencies
}

// RecordMessageBusOperation records metrics for a message bus operation
func (m *MetricsCollector) RecordMessageBusOperation(operation string, success bool, latency time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Increment message bus counter
	m.messageBusCounts[operation]++

	// Record operation-specific counters
	switch operation {
	case MessageBusOperationSend:
		m.counters[CounterMessagesSent]++
	case MessageBusOperationReceive:
		m.counters[CounterMessagesReceived]++
	case MessageBusOperationComplete:
		m.counters[CounterMessagesProcessed]++
	}

	if !success {
		m.counters[CounterMessagesError]++
		m.errorCounts[ErrorTypeMessageBus]++
	}

	// Record latency
	latencies, ok := m.messageBusLatencies[operation]
	if !ok {
		latencies = make([]time.Duration, 0, m.maxHistogramSamples)
	}
	if len(latencies) >= m.maxHistogramSamples {
		latencies = latencies[1:]
	}
	latencies = append(latencies, latency)
	m.messageBusLatencies[operation] = latencies
}

// RecordDatabaseQuery records metrics for a database query
func (m *MetricsCollector) RecordDatabaseQuery(queryType string, success bool, latency time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Increment database query counter
	m.databaseQueryCounts[queryType]++
	m.counters[CounterDBQueriesTotal]++

	if !success {
		m.counters[CounterDBQueriesError]++
		m.errorCounts[ErrorTypeDatabase]++
	}

	// Record latency
	latencies, ok := m.databaseLatencies[queryType]
	if !ok {
		latencies = make([]time.Duration, 0, m.maxHistogramSamples)
	}
	if len(latencies) >= m.maxHistogramSamples {
		latencies = latencies[1:]
	}
	latencies = append(latencies, latency)
	m.databaseLatencies[queryType] = latencies
}

// RecordError records an error of the given type
func (m *MetricsCollector) RecordError(errorType string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.errorCounts[errorType]++
	m.counters[CounterErrorsTotal]++
}

// SetActiveOperations sets tber of active operations
func (m *MetricsCollector) SetActiveOperations(count int) {
	m.SetGauge(GaugeActiveOperations, float64(count))
}

// SetActiveOperationGroups sets the number of active operation groups
func (m *MetricsCollector) SetActiveOperationGroups(count int) {
	m.SetGauge(GaugeActiveOperationGroups, float64(count))
}

// SetPendingMessages sets the number of pending messages
func (m *MetricsCollector) SetPendingMessages(count int) {
	m.SetGauge(GaugePendingMessages, float64(count))
}

// GetMetrics returns all collected metrics in a structured format
func (m *MetricsCollector) GetMetrics() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Calculate average latencies
	httpLatencies := make(map[string]float64)
	for path, latencies := range m.requestLatencies {
		if len(latencies) > 0 {
			var sum time.Duration
			for _, l := range latencies {
				sum += l
			}
			httpLatencies[path] = float64(sum.Milliseconds()) / float64(len(latencies))
		}
	}

	operationLatencies := make(map[string]float64)
	for opType, latencies := range m.operationLatencies {
		if len(latencies) > 0 {
			var sum time.Duration
			for _, l := range latencies {
				sum += l
			}
			operationLatencies[opType] = float64(sum.Milliseconds()) / float64(len(latencies))
		}
	}

	messageBusLatencies := make(map[string]float64)
	for opType, latencies := range m.messageBusLatencies {
		if len(latencies) > 0 {
			var sum time.Duration
			for _, l := range latencies {
				sum += l
			}
			messageBusLatencies[opType] = float64(sum.Milliseconds()) / float64(len(latencies))
		}
	}

	databaseLatencies := make(map[string]float64)
	for queryType, latencies := range m.databaseLatencies {
		if len(latencies) > 0 {
			var sum time.Duration
			for _, l := range latencies {
				sum += l
			}
			databaseLatencies[queryType] = float64(sum.Milliseconds()) / float64(len(latencies))
		}
	}

	// Calculate uptime
	uptime := time.Since(m.startTime)

	return map[string]interface{}{
		"uptime_seconds":           uptime.Seconds(),
		"counters":                 m.counters,
		"gauges":                   m.gauges,
		"request_counts":           m.requestCounts,
		"request_latencies_ms":     httpLatencies,
		"operation_counts":         m.operationCounts,
		"operation_latencies_ms":   operationLatencies,
		"message_bus_counts":       m.messageBusCounts,
		"message_bus_latencies_ms": messageBusLatencies,
		"database_query_counts":    m.databaseQueryCounts,
		"database_latencies_ms":    databaseLatencies,
		"error_counts":             m.errorCounts,
	}
}

// GetHealthStatus returns a simple health status based on metrics
func (m *MetricsCollector) GetHealthStatus() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Determine overall health
	healthy := true

	// Check if there are too many errors
	errorRate := 0.0
	totalRequests := m.counters[CounterHTTPRequests]
	if totalRequests > 0 {
		errorRate = float64(m.counters[CounterHTTPRequestsError]) / float64(totalRequests)
	}

	// Define thresholds for health status
	const errorRateThreshold = 0.05 // 5% error rate is considered unhealthy

	if errorRate > errorRateThreshold {
		healthy = false
	}

	// Get uptime
	uptime := time.Since(m.startTime)

	return map[string]interface{}{
		"status": map[string]interface{}{
			"healthy":        healthy,
			"uptime_seconds": uptime.Seconds(),
		},
		"metrics": map[string]interface{}{
			"total_requests":       totalRequests,
			"error_rate":           errorRate,
			"operations_completed": m.counters[CounterOperationsCompleted],
			"operations_failed":    m.counters[CounterOperationsFailed],
			"messages_processed":   m.counters[CounterMessagesProcessed],
			"messages_error":       m.counters[CounterMessagesError],
		},
	}
}

// Global metrics collector instance
var globalCollector *MetricsCollector
var once sync.Once

// GetMetricsCollector returns the global metrics collector instance
func GetMetricsCollector() *MetricsCollector {
	once.Do(func() {
		globalCollector = NewMetricsCollector()
	})
	return globalCollector
}
