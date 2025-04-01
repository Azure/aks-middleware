package otelaudit

import (
	"bytes"
	"net/http"
	"time"

	"log/slog"
)

// responseWriter buffers the output and status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	buf        *bytes.Buffer
}

func (w *responseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *responseWriter) Write(b []byte) (int, error) {
	if w.buf == nil {
		w.buf = new(bytes.Buffer)
	}
	w.buf.Write(b)
	if w.statusCode == 0 {
		w.statusCode = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

// Middleware implements a separate OTEL audit middleware.
type Middleware struct {
	next       http.Handler
	now        func() time.Time
	logger     *slog.Logger
	otelConfig *OtelConfig
}

// ServeHTTP wraps the request and triggers the audit event.
func (m *Middleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	customWriter := &responseWriter{
		ResponseWriter: w,
		buf:            new(bytes.Buffer),
	}

	ctx := r.Context()

	m.next.ServeHTTP(customWriter, r.WithContext(ctx))

	// If error, extract the buffered response body as error message.
	errorMsg := ""
	if customWriter.statusCode >= http.StatusBadRequest {
		errorMsg = customWriter.buf.String()
	}

	// Call the OTEL audit event sender.
	SendOtelAuditEvent(m.logger, m.otelConfig, ctx, customWriter.statusCode, r, errorMsg)
}

// NewMiddleware returns a middleware function to add OTEL audit logging.
func NewMiddleware(logger *slog.Logger, otelConfig *OtelConfig) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return &Middleware{
			next:       next,
			now:        time.Now,
			logger:     logger,
			otelConfig: otelConfig,
		}
	}
}
