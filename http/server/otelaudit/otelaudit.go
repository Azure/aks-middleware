package otelaudit

import (
	"net/http"

	"log/slog"

	"github.com/Azure/aks-middleware/http/common/logging"
)

// Middleware implements a separate OTEL audit middleware.
type Middleware struct {
	next       http.Handler
	logger     *slog.Logger
	otelConfig *OtelConfig
}

// ServeHTTP wraps the request and triggers the audit event.
func (m *Middleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	customWriter := logging.NewResponseWriter(w)

	ctx := r.Context()
	m.next.ServeHTTP(customWriter, r)
	errorMsg := customWriter.Buf.String()

	// Call the OTEL audit event sender.
	SendOtelAuditEvent(m.logger, m.otelConfig, ctx, customWriter.StatusCode, r, errorMsg)
}

// NewMiddleware returns a middleware function to add OTEL audit logging.
func NewOtelAuditLogging(logger *slog.Logger, otelConfig *OtelConfig) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return &Middleware{
			next:       next,
			logger:     logger,
			otelConfig: otelConfig,
		}
	}
}
