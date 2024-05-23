package httpmw

import (
	"net/http"
	"time"

	"github.com/gorilla/mux"

	"github.com/Azure/aks-middleware/ctxlogger"
)

func NewLogging() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return &loggingMiddleware{
			next: next,
			now:  time.Now,
		}
	}
}

var _ http.Handler = &loggingMiddleware{}

type loggingMiddleware struct {
	next http.Handler
	now  func() time.Time
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
	ctx := r.Context()
	logger := ctxlogger.GetLogger(ctx)

	customWriter := &responseWriter{ResponseWriter: w}

	startTime := l.now()
	l.next.ServeHTTP(customWriter, r)
	endTime := l.now()

	latency := endTime.Sub(startTime)

	logger.With(
		"status", customWriter.statusCode,
		"latency", latency.Milliseconds(),
	).Info("finished call")
}
