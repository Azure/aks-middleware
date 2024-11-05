package logging

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

type contextKey string

const attrsKey contextKey = "slogAttrs"

const (
	//these should be in some arm package?
	RequestAcsOperationIDHeader = "x-ms-acs-operation-id"
	RequestCorrelationIDHeader  = "x-ms-correlation-request-id"
)

type contextHandler struct {
	handler slog.Handler
}

func (h *contextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

func (h *contextHandler) Handle(ctx context.Context, record slog.Record) error {
	// Extract attributes from context
	if attrs, ok := ctx.Value(attrsKey).([]slog.Attr); ok {
		record.AddAttrs(attrs...)
	}
	return h.handler.Handle(ctx, record)
}

func (h *contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &contextHandler{handler: h.handler.WithAttrs(attrs)}
}

func (h *contextHandler) WithGroup(name string) slog.Handler {
	return &contextHandler{handler: h.handler.WithGroup(name)}
}

// TODO (Tom): Add a logger wrapper in its own package
// https://medium.com/@ansujain/building-a-logger-wrapper-in-go-with-support-for-multiple-logging-libraries-48092b826bee

// more info about http handler here: https://pkg.go.dev/net/http#Handler
func NewLogging(logger *slog.Logger) mux.MiddlewareFunc {
	// Wrap the existing handler with the context handler
	logger = slog.New(&contextHandler{handler: logger.Handler()})

	return func(next http.Handler) http.Handler {
		return &loggingMiddleware{
			next:   next,
			now:    time.Now,
			logger: *logger,
		}
	}
}

// enforcing that loggingMiddleware implements the http.Handler interface to ensure safety at compile time
var _ http.Handler = &loggingMiddleware{}

type loggingMiddleware struct {
	next   http.Handler
	now    func() time.Time
	logger slog.Logger
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *responseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *responseWriter) Write(b []byte) (int, error) {
	if w.statusCode == 0 {
		w.statusCode = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

func WithAttrs(ctx context.Context, attrs ...slog.Attr) context.Context {
	existingAttrs, _ := ctx.Value(attrsKey).([]slog.Attr)
	allAttrs := append(existingAttrs, attrs...)
	return context.WithValue(ctx, attrsKey, allAttrs)
}

func (l *loggingMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	customWriter := &responseWriter{ResponseWriter: w}

	startTime := l.now()

	ctx := r.Context()
	ctx = WithAttrs(ctx,
		slog.String("operationID", r.Header.Get(RequestAcsOperationIDHeader)),
		slog.String("correlationID", r.Header.Get(RequestCorrelationIDHeader)),
	)

	l.LogRequestStart(ctx, r)
	l.next.ServeHTTP(customWriter, r.WithContext(ctx))
	endTime := l.now()

	latency := endTime.Sub(startTime)
	l.LogRequestEnd(ctx, r, "RequestEnd", customWriter.statusCode, latency)
	l.LogRequestEnd(ctx, r, "finished call", customWriter.statusCode, latency)
}

func (l *loggingMiddleware) LogRequestStart(ctx context.Context, r *http.Request) {
	l.logger.InfoContext(ctx, "RequestStart", "source", "ApiRequestLog", "protocol", "HTTP", "method_type", "unary",
		"component", "client", "method", r.Method, "service", r.Host, "url", r.URL.String())
}

func (l *loggingMiddleware) LogRequestEnd(ctx context.Context, r *http.Request, msg string, statusCode int, duration time.Duration) {
	l.logger.InfoContext(ctx, msg, "source", "ApiRequestLog", "protocol", "HTTP", "method_type",
		"unary", "component", "client", "method", r.Method, "service", r.Host,
		"url", r.URL.String(), "code", statusCode, "time_ms", duration.Milliseconds())
}
