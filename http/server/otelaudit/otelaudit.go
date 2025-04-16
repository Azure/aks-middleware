package otelaudit

import (
	"bytes"
	"net/http"
	"time"

	"log/slog"

	"github.com/Azure/aks-middleware/http/common/logging"
)

// Middleware implements a separate OTEL audit middleware.
type Middleware struct {
	next       http.Handler
	now        func() time.Time
	logger     *slog.Logger
	otelConfig *OtelConfig
}

// ServeHTTP wraps the request and triggers the audit event.
func (m *Middleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	customWriter := &logging.ResponseWriter{
		ResponseWriter: w,
		Buf:            new(bytes.Buffer),
	}

	ctx := r.Context()

	m.next.ServeHTTP(customWriter, r.WithContext(ctx))

	// If error, extract the buffered response body as error message.
	errorMsg := ""
	if customWriter.StatusCode >= http.StatusBadRequest {
		errorMsg = customWriter.Buf.String()
	}

	// Call the OTEL audit event sender.
	SendOtelAuditEvent(m.logger, m.otelConfig, ctx, customWriter.StatusCode, r, errorMsg)
}

// NewMiddleware returns a middleware function to add OTEL audit logging.
func NewOtelAuditLogging(logger *slog.Logger, otelConfig *OtelConfig) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return &Middleware{
			next:       next,
			now:        time.Now,
			logger:     logger,
			otelConfig: otelConfig,
		}
	}
}
