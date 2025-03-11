package logging

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"google.golang.org/grpc/metadata"
)

// TODO (Tom): Add a logger wrapper in its own package
// https://medium.com/@ansujain/building-a-logger-wrapper-in-go-with-support-for-multiple-logging-libraries-48092b826bee

// more info about http handler here: https://pkg.go.dev/net/http#Handler
type loggingfunc func(w http.ResponseWriter, r *http.Request, extraAttributes map[string]interface{}) map[string]interface{}
type CustomAttributes struct {
	customAttributes     map[string]interface{}
	attributeInitializer *loggingfunc // sets keys for custom attributes at the beginning of ServeHTTP()
	attributeAssigner    *loggingfunc // assigns values for custom attributes after request has completed
}

func NewLogging(logger *slog.Logger, customAttributeAssigner *CustomAttributes) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return &loggingMiddleware{
			next:                    next,
			now:                     time.Now,
			logger:                  *logger,
			customAttributeAssigner: *customAttributeAssigner,
		}
	}
}

// enforcing that loggingMiddleware implements the http.Handler interface to ensure safety at compile time
var _ http.Handler = &loggingMiddleware{}

type loggingMiddleware struct {
	next                    http.Handler
	now                     func() time.Time
	logger                  slog.Logger
	source                  string
	customAttributeAssigner CustomAttributes
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

func (l *loggingMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	extraAttributes := (*l.customAttributeAssigner.attributeInitializer)(w, r, l.customAttributeAssigner.customAttributes)
	customWriter := &responseWriter{ResponseWriter: w}
	startTime := l.now()
	ctx := r.Context()

	l.LogRequestStart(ctx, r, "RequestStart", extraAttributes)
	l.next.ServeHTTP(customWriter, r.WithContext(ctx))
	endTime := l.now()

	latency := endTime.Sub(startTime)
	extraAttributes = (*l.customAttributeAssigner.attributeAssigner)(w, r, l.customAttributeAssigner.customAttributes)
	l.LogRequestEnd(ctx, r, "RequestEnd", customWriter.statusCode, latency, extraAttributes)
	l.LogRequestEnd(ctx, r, "finished call", customWriter.statusCode, latency, extraAttributes)
}

func (l *loggingMiddleware) BuildLoggingAttributes(ctx context.Context, r *http.Request, extra ...interface{}) []interface{} {
	if len(l.source) == 0 {
		l.source = "ApiRequestLog"
	}
	return BuildAttributes(ctx, r, l.source, extra)
}

func BuildAttributes(ctx context.Context, r *http.Request, source string, extra ...interface{}) []interface{} {
	md, ok := metadata.FromIncomingContext(ctx)
	attributes := []interface{}{
		"source", source,
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

func (l *loggingMiddleware) LogRequestStart(ctx context.Context, r *http.Request, msg string, extraAttributes map[string]interface{}) {
	attributes := l.BuildLoggingAttributes(ctx, r, extraAttributes)
	l.logger.InfoContext(ctx, msg, attributes...)
}

func (l *loggingMiddleware) LogRequestEnd(ctx context.Context, r *http.Request, msg string, statusCode int, duration time.Duration, extraAttributes map[string]interface{}) {
	attributes := l.BuildLoggingAttributes(ctx, r, "code", statusCode, "time_ms", duration.Milliseconds(), extraAttributes)
	l.logger.InfoContext(ctx, msg, attributes...)
}
