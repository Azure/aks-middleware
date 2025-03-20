package logging

import (
    "bytes"
    "context"
    "log/slog"
    "net/http"
    "time"

    "github.com/gorilla/mux"
    "google.golang.org/grpc/metadata"
)

// Modify the responseWriter to buffer written data.
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

func NewLogging(logger *slog.Logger, otelConfig *OtelConfig) mux.MiddlewareFunc {
    return func(next http.Handler) http.Handler {
        return &loggingMiddleware{
            next:       next,
            now:        time.Now,
            logger:     logger,
            otelConfig: otelConfig,
        }
    }
}

// enforcing that loggingMiddleware implements the http.Handler interface to ensure safety at compile time
var _ http.Handler = &loggingMiddleware{}

type loggingMiddleware struct {
    next       http.Handler
    now        func() time.Time
    logger     *slog.Logger
    otelConfig *OtelConfig
}

func (l *loggingMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    customWriter := &responseWriter{
        ResponseWriter: w,
        buf:            new(bytes.Buffer),
    }

    startTime := l.now()
    ctx := r.Context()

    l.LogRequestStart(ctx, r, "RequestStart")
    l.next.ServeHTTP(customWriter, r.WithContext(ctx))
    endTime := l.now()

    latency := endTime.Sub(startTime)

	// If error, extract the buffered response body as error message.
    errorMsg := ""
    if customWriter.statusCode >= http.StatusBadRequest {
        errorMsg = customWriter.buf.String()
    }

    l.LogRequestEnd(ctx, r, "RequestEnd", customWriter.statusCode, latency, errorMsg)
    l.LogRequestEnd(ctx, r, "finished call", customWriter.statusCode, latency, errorMsg)

    
    l.sendOtelAuditEvent(ctx, customWriter.statusCode, r, errorMsg)
}

func BuildAttributes(ctx context.Context, r *http.Request, extra ...interface{}) []interface{} {
    md, ok := metadata.FromIncomingContext(ctx)
    attributes := []interface{}{
        "source", "ApiRequestLog",
        "protocol", "HTTP",
        "method_type", "unary",
        "component", "server",
        "method", r.Method,
        "service", r.Host,
        "url", r.URL.String(),
    }

    headers := make(map[string]string)
    if ok {
        for key, values := range md {
            if len(values) > 0 {
                headers[key] = values[0]
            }
        }
    }

    attributes = append(attributes, "headers", headers)
    attributes = append(attributes, extra...)
    return attributes
}

func (l *loggingMiddleware) LogRequestStart(ctx context.Context, r *http.Request, msg string) {
    attributes := BuildAttributes(ctx, r)
    l.logger.InfoContext(ctx, msg, attributes...)
}

func (l *loggingMiddleware) LogRequestEnd(ctx context.Context, r *http.Request, msg string, statusCode int, duration time.Duration, err string) {
    attributes := BuildAttributes(ctx, r, "code", statusCode, "time_ms", duration.Milliseconds(), "error", err)
    l.logger.InfoContext(ctx, msg, attributes...)
}