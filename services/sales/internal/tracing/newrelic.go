package tracing

import (
	"example.com/backstage/services/sales/config"

	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// Tracer defines the interface for tracing
type Tracer interface {
	StartTransaction(name string) *newrelic.Transaction
	StartSpan(name string, transaction *newrelic.Transaction) *newrelic.Segment
	EndTransaction(transaction *newrelic.Transaction)
	StartExternalSegment(txn *newrelic.Transaction, req *newrelic.ExternalSegment) *newrelic.ExternalSegment
	RecordError(txn *newrelic.Transaction, err error)
	AddAttribute(txn *newrelic.Transaction, key string, value interface{})
	Close()
}

// NewRelicTracer implements Tracer using New Relic
type NewRelicTracer struct {
	app        *newrelic.Application
	appName    string
	license    string
	logForward bool
	enabled    bool
}

// NewTracer creates a new tracer
func NewTracer(config config.TracingConfig) (Tracer, error) {
	if config.LicenseKey == "" {
		log.Warn().Msg("New Relic license key not provided, tracing will be disabled")
		return &NewRelicTracer{enabled: false}, nil
	}

	app, err := newrelic.NewApplication(
		newrelic.ConfigAppName(config.AppName),
		newrelic.ConfigLicense(config.LicenseKey),
		newrelic.ConfigDistributedTracerEnabled(config.DistribTracing),
		newrelic.ConfigAppLogForwardingEnabled(config.LogEnabled),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize New Relic")
	}

	return &NewRelicTracer{
		app:        app,
		appName:    config.AppName,
		license:    config.LicenseKey,
		logForward: config.LogEnabled,
		enabled:    true,
	}, nil
}

// StartTransaction starts a new transaction
func (t *NewRelicTracer) StartTransaction(name string) *newrelic.Transaction {
	if !t.enabled || t.app == nil {
		return nil
	}
	return t.app.StartTransaction(name)
}

// StartSpan starts a new segment within a transaction
func (t *NewRelicTracer) StartSpan(name string, transaction *newrelic.Transaction) *newrelic.Segment {
	if !t.enabled || transaction == nil {
		return &newrelic.Segment{}
	}
	return transaction.StartSegment(name)
}

// EndTransaction ends a transaction
func (t *NewRelicTracer) EndTransaction(transaction *newrelic.Transaction) {
	if !t.enabled || transaction == nil {
		return
	}
	transaction.End()
}

// StartExternalSegment starts an external service segment
func (t *NewRelicTracer) StartExternalSegment(txn *newrelic.Transaction, req *newrelic.ExternalSegment) *newrelic.ExternalSegment {
	if !t.enabled || txn == nil {
		return &newrelic.ExternalSegment{}
	}
	return req
}

// RecordError records an error in a transaction
func (t *NewRelicTracer) RecordError(txn *newrelic.Transaction, err error) {
	if !t.enabled || txn == nil || err == nil {
		return
	}
	txn.NoticeError(err)
}

// AddAttribute adds an attribute to a transaction
func (t *NewRelicTracer) AddAttribute(txn *newrelic.Transaction, key string, value interface{}) {
	if !t.enabled || txn == nil {
		return
	}
	txn.AddAttribute(key, value)
}

// Close gracefully shuts down the tracer
func (t *NewRelicTracer) Close() {
	if !t.enabled || t.app == nil {
		return
	}
	
	// New Relic's application has no explicit Close method
	// This is just a placeholder for future maintenance
	log.Info().Msg("New Relic tracer shutdown")
}