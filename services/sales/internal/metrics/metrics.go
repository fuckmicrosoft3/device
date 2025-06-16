package metrics

import (
	"sync"
	"sync/atomic"
	"time"
)

// MetricType defines types of metrics we track
type MetricType string


// Different metric types
const (
	TypeCounter     MetricType = "counter"     // Always increasing count
	TypeGauge       MetricType = "gauge"       // Point-in-time value
	TypeTimer       MetricType = "timer"       // Duration measurement
	TypeErrorRate   MetricType = "error_rate"  // Error percentage
	TypeHealthCheck MetricType = "health"      // Health status (0/1)
)

// MetricValue represents a metric value with metadata
type MetricValue struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Type        MetricType `json:"type"`
	Value       int64      `json:"value"`
	Tags        map[string]string `json:"tags,omitempty"`
}

// TimerMetric captures timing information
type TimerMetric struct {
	Count         int64   `json:"count"`
	TotalTimeMs   int64   `json:"total_time_ms"`
	AverageTimeMs float64 `json:"average_time_ms"`
	MinTimeMs     int64   `json:"min_time_ms"`
	MaxTimeMs     int64   `json:"max_time_ms"`
}

// ErrorRateMetric captures error rates
type ErrorRateMetric struct {
	Total      int64   `json:"total"`
	Errors     int64   `json:"errors"`
	ErrorRate  float64 `json:"error_rate"`
}

// Metrics is the main metrics collector
type Metrics struct {
	mu            sync.RWMutex
	counters      map[string]*int64
	gauges        map[string]*int64
	timers        map[string]*struct {
		count       int64
		totalTimeMs int64
		minTimeMs   int64
		maxTimeMs   int64
	}
	errorRates    map[string]*struct {
		total  int64
		errors int64
	}
	healthChecks  map[string]*int64
	startTime     time.Time
}

// NewMetrics creates a new metrics collector
func NewMetrics() *Metrics {
	return &Metrics{
		counters:     make(map[string]*int64),
		gauges:       make(map[string]*int64),
		timers:       make(map[string]*struct {
			count       int64
			totalTimeMs int64
			minTimeMs   int64
			maxTimeMs   int64
		}),
		errorRates:   make(map[string]*struct {
			total  int64
			errors int64
		}),
		healthChecks: make(map[string]*int64),
		startTime:    time.Now(),
	}
}

// IncrementCounter increments a counter by 1
func (m *Metrics) IncrementCounter(name string) {
	m.IncrementCounterBy(name, 1)
}

// IncrementCounterBy increments a counter by the specified value
func (m *Metrics) IncrementCounterBy(name string, value int64) {
	m.mu.RLock()
	counter, exists := m.counters[name]
	m.mu.RUnlock()

	if !exists {
		m.mu.Lock()
		// Check again to avoid race conditions
		if counter, exists = m.counters[name]; !exists {
			var c int64
			counter = &c
			m.counters[name] = counter
		}
		m.mu.Unlock()
	}

	atomic.AddInt64(counter, value)
}

// SetGauge sets a gauge to a specific value
func (m *Metrics) SetGauge(name string, value int64) {
	m.mu.RLock()
	gauge, exists := m.gauges[name]
	m.mu.RUnlock()

	if !exists {
		m.mu.Lock()
		if gauge, exists = m.gauges[name]; !exists {
			var g int64
			gauge = &g
			m.gauges[name] = gauge
		}
		m.mu.Unlock()
	}

	atomic.StoreInt64(gauge, value)
}

// RecordTimer records a timing measurement
func (m *Metrics) RecordTimer(name string, durationMs int64) {
	m.mu.RLock()
	timer, exists := m.timers[name]
	m.mu.RUnlock()

	if !exists {
		m.mu.Lock()
		if timer, exists = m.timers[name]; !exists {
			timer = &struct {
				count       int64
				totalTimeMs int64
				minTimeMs   int64
				maxTimeMs   int64
			}{
				minTimeMs: 9223372036854775807, // Max int64
			}
			m.timers[name] = timer
		}
		m.mu.Unlock()
	}

	atomic.AddInt64(&timer.count, 1)
	atomic.AddInt64(&timer.totalTimeMs, durationMs)

	// Update min if smaller
	for {
		currentMin := atomic.LoadInt64(&timer.minTimeMs)
		if durationMs >= currentMin {
			break
		}
		if atomic.CompareAndSwapInt64(&timer.minTimeMs, currentMin, durationMs) {
			break
		}
	}

	// Update max if larger
	for {
		currentMax := atomic.LoadInt64(&timer.maxTimeMs)
		if durationMs <= currentMax {
			break
		}
		if atomic.CompareAndSwapInt64(&timer.maxTimeMs, currentMax, durationMs) {
			break
		}
	}
}

// RecordSuccess records a successful operation for error rate tracking
func (m *Metrics) RecordSuccess(name string) {
	m.recordErrorRate(name, false)
}

// RecordError records an error for error rate tracking
func (m *Metrics) RecordError(name string) {
	m.recordErrorRate(name, true)
}

// recordErrorRate records a success or error for error rate calculation
func (m *Metrics) recordErrorRate(name string, isError bool) {
	m.mu.RLock()
	errorRate, exists := m.errorRates[name]
	m.mu.RUnlock()

	if !exists {
		m.mu.Lock()
		if errorRate, exists = m.errorRates[name]; !exists {
			errorRate = &struct {
				total  int64
				errors int64
			}{}
			m.errorRates[name] = errorRate
		}
		m.mu.Unlock()
	}

	atomic.AddInt64(&errorRate.total, 1)
	if isError {
		atomic.AddInt64(&errorRate.errors, 1)
	}
}

// SetHealth sets the health status of a component (0 = unhealthy, 1 = healthy)
func (m *Metrics) SetHealth(component string, isHealthy bool) {
	var value int64
	if isHealthy {
		value = 1
	} else {
		value = 0
	}

	m.mu.RLock()
	health, exists := m.healthChecks[component]
	m.mu.RUnlock()

	if !exists {
		m.mu.Lock()
		if health, exists = m.healthChecks[component]; !exists {
			var h int64
			health = &h
			m.healthChecks[component] = health
		}
		m.mu.Unlock()
	}

	atomic.StoreInt64(health, value)
}

// GetCounters returns all counters
func (m *Metrics) GetCounters() map[string]int64 {
	counters := make(map[string]int64)
	
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	for name, counter := range m.counters {
		counters[name] = atomic.LoadInt64(counter)
	}
	
	return counters
}

// GetGauges returns all gauges
func (m *Metrics) GetGauges() map[string]int64 {
	gauges := make(map[string]int64)
	
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	for name, gauge := range m.gauges {
		gauges[name] = atomic.LoadInt64(gauge)
	}
	
	return gauges
}

// GetTimers returns all timers
func (m *Metrics) GetTimers() map[string]TimerMetric {
	timers := make(map[string]TimerMetric)
	
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	for name, timer := range m.timers {
		count := atomic.LoadInt64(&timer.count)
		totalTime := atomic.LoadInt64(&timer.totalTimeMs)
		
		var average float64
		if count > 0 {
			average = float64(totalTime) / float64(count)
		}
		
		timers[name] = TimerMetric{
			Count:         count,
			TotalTimeMs:   totalTime,
			AverageTimeMs: average,
			MinTimeMs:     atomic.LoadInt64(&timer.minTimeMs),
			MaxTimeMs:     atomic.LoadInt64(&timer.maxTimeMs),
		}
	}
	
	return timers
}

// GetErrorRates returns all error rates
func (m *Metrics) GetErrorRates() map[string]ErrorRateMetric {
	errorRates := make(map[string]ErrorRateMetric)
	
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	for name, er := range m.errorRates {
		total := atomic.LoadInt64(&er.total)
		errors := atomic.LoadInt64(&er.errors)
		
		var rate float64
		if total > 0 {
			rate = float64(errors) / float64(total) * 100.0
		}
		
		errorRates[name] = ErrorRateMetric{
			Total:     total,
			Errors:    errors,
			ErrorRate: rate,
		}
	}
	
	return errorRates
}

// GetHealthChecks returns all health checks
func (m *Metrics) GetHealthChecks() map[string]bool {
	checks := make(map[string]bool)
	
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	for name, health := range m.healthChecks {
		checks[name] = atomic.LoadInt64(health) > 0
	}
	
	return checks
}

// GetUptimeSeconds returns the service uptime in seconds
func (m *Metrics) GetUptimeSeconds() int64 {
	return int64(time.Since(m.startTime).Seconds())
}

// GetAllMetrics returns all metrics in a structured format
func (m *Metrics) GetAllMetrics() map[string]interface{} {
	return map[string]interface{}{
		"uptime_seconds": m.GetUptimeSeconds(),
		"counters":       m.GetCounters(),
		"gauges":         m.GetGauges(),
		"timers":         m.GetTimers(),
		"error_rates":    m.GetErrorRates(),
		"health_checks":  m.GetHealthChecks(),
	}
}