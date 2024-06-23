package httpmw

import (
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

type Logger interface {
	Info(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
}

func NewLogging(logger Logger) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return &loggingMiddleware{
			next:   next,
			now:    time.Now,
			logger: logger,
		}
	}
}

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

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.statusCode == 0 {
		rw.statusCode = http.StatusOK
	}
	return rw.ResponseWriter.Write(b)
}

func (l *loggingMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	customWriter := &responseWriter{ResponseWriter: w}

	startTime := l.now()
	l.LogRequestStart(r)
	l.next.ServeHTTP(customWriter, r)
	endTime := l.now()

	latency := endTime.Sub(startTime)
	l.LogRequestEnd(r, customWriter.statusCode, latency)

	l.logger.Info("finished call", "status", customWriter.statusCode, "latency", latency.Milliseconds())
}

func (l *loggingMiddleware) LogRequestStart(r *http.Request) {
	l.logger.Info("request started", "method", r.Method, "url", r.URL.String())
}

func (l *loggingMiddleware) LogRequestEnd(r *http.Request, statusCode int, duration time.Duration) {
	l.logger.Info("request ended", "method", r.Method, "url", r.URL.String(), "status", statusCode, "duration", duration.Milliseconds())
}
