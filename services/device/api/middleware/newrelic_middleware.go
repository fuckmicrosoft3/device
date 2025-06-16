package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/newrelic/go-agent/v3/integrations/nrgin"
)

// NewRelicMiddleware returns a gin middleware for New Relic tracing
func NewRelicMiddleware(app *newrelic.Application) gin.HandlerFunc {
	return nrgin.Middleware(app)
}
