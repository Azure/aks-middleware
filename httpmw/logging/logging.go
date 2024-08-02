package logging

import (
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

type Logger interface {
	Info(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
}

// more info about http handler here: https://pkg.go.dev/net/http#Handler
func NewLogging(logger Logger) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return &loggingMiddleware{
			next:   next,
			now:    time.Now,
			logger: logger,
		}
	}
}

// enforcing that loggingMiddleware implements the http.Handler interface to ensure safety at compile time
var _ http.Handler = &loggingMiddleware{}

type loggingMiddleware struct {
	next   http.Handler
	now    func() time.Time
	logger Logger
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
	customWriter := &responseWriter{ResponseWriter: w}

	startTime := l.now()
	l.LogRequestStart(r)
	l.next.ServeHTTP(customWriter, r)
	endTime := l.now()

	latency := endTime.Sub(startTime)
	l.LogRequestEnd(r, "RequestEnd", customWriter.statusCode, latency)
	l.LogRequestEnd(r, "finished call", customWriter.statusCode, latency)
}

func (l *loggingMiddleware) LogRequestStart(r *http.Request) {
	l.logger.Info("RequestStart", "source", "ApiRequestLog", "protocol", "HTTP", "method_type", "unary",
		"component", "client", "method", r.Method, "service", r.Host, "url", r.URL.String())
}

func (l *loggingMiddleware) LogRequestEnd(r *http.Request, msg string, statusCode int, duration time.Duration) {
	l.logger.Info(msg, "source", "ApiRequestLog", "protocol", "HTTP", "method_type", "unary",
		"component", "client", "method", r.Method, "service", r.Host, "url", r.URL.String(),
		"code", statusCode, "time_ms", duration.Milliseconds())
}
