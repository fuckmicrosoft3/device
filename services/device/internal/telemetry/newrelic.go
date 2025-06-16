package telemetry

import (
	"time"
	
	"example.com/backstage/services/device/config"
	
	"github.com/newrelic/go-agent/v3/newrelic"
)

// InitNewRelic initializes the New Relic application
func InitNewRelic(cfg config.NewRelicConfig) (*newrelic.Application, error) {
	if !cfg.Enabled || cfg.LicenseKey == "" {
		return nil, nil
	}
	
	app, err := newrelic.NewApplication(
		newrelic.ConfigAppName(cfg.AppName),
		newrelic.ConfigLicense(cfg.LicenseKey),
		newrelic.ConfigDistributedTracerEnabled(true),
		newrelic.ConfigAppLogForwardingEnabled(true),
		newrelic.ConfigAppLogEnabled(true),
		newrelic.ConfigAppLogForwardingEnabled(true),
	)
	
	if err != nil {
		return nil, err
	}
	
	// Wait for the application to connect
	if err := app.WaitForConnection(5 * time.Second); err != nil {
		return nil, err
	}
	
	return app, nil
}
