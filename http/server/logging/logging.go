package logging

import (
	"context"
	"encoding/json"
	log "log/slog"
	"net/http"
	"os"
	"time"

	"github.com/Azure/aks-middleware/http/common/logging"
	"github.com/gorilla/mux"
	"google.golang.org/grpc/metadata"
)

// TODO (Tom): Add a logger wrapper in its own package
// https://medium.com/@ansujain/building-a-logger-wrapper-in-go-with-support-for-multiple-logging-libraries-48092b826bee

const (
	apiRequestLogSource = "ApiRequestLog"
)

// more info about http handler here: https://pkg.go.dev/net/http#Handler
func NewLogging(logger *log.Logger) mux.MiddlewareFunc {
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
	logger *log.Logger
}
type RequestLogData struct {
	Code     int
	Duration time.Duration
	Error    string
}

func (l *loggingMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	customWriter := logging.NewResponseWriter(w)

	startTime := l.now()
	ctx := r.Context()

	l.LogRequestStart(ctx, r, "RequestStart")
	l.next.ServeHTTP(customWriter, r)
	endTime := l.now()

	latency := endTime.Sub(startTime)
	errorMsg := customWriter.Buf.String()

	data := RequestLogData{
		Code:     customWriter.StatusCode,
		Duration: latency,
		Error:    errorMsg,
	}
	// TODO (tomabraham): move RequestStart and RequestEnd to a different interceptor
	// ApiRequestLog should only get "finished call" logs
	l.LogRequestEnd(ctx, r, "RequestEnd", data)
	l.LogRequestEnd(ctx, r, "finished call", data)
}

// Headers are json.Marshaled, but if that fails, the headers are sent to kusto as the original map[string]string
// The function logs to ApiRequestLog to notify users of the error if they wish to fix it
func BuildAttributes(ctx context.Context, r *http.Request, extra ...interface{}) []interface{} {
	md, ok := metadata.FromIncomingContext(ctx)
	attributes := []interface{}{
		"source", "ApiRequestLog",
		"protocol", "HTTP",
		"method_type", "unary",
		"component", "server",
		"method", logging.GetMethodInfo(r.Method, r.URL.Path),
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

	templogger := log.New(log.NewJSONHandler(os.Stdout, nil)).With("source", apiRequestLogSource)
	headerBytes, err := json.Marshal(headers)
	headersStr := string(headerBytes)

	var finalHeaders interface{}
	if err != nil {
		templogger.Error("error marshaling headers", "error", err)
		finalHeaders = headers
	} else {
		finalHeaders = headersStr
	}

	attributes = append(attributes, "headers", finalHeaders)
	attributes = append(attributes, extra...)
	return attributes
}

func (l *loggingMiddleware) LogRequestStart(ctx context.Context, r *http.Request, msg string) {
	attributes := BuildAttributes(ctx, r)
	l.logger.InfoContext(ctx, msg, attributes...)
}

func (l *loggingMiddleware) LogRequestEnd(ctx context.Context, r *http.Request, msg string, data RequestLogData) {
	attributes := BuildAttributes(ctx, r, "code", data.Code, "time_ms", data.Duration.Milliseconds(), "error", data.Error)
	if data.Code >= http.StatusBadRequest {
		l.logger.ErrorContext(ctx, msg, attributes...)
	} else {
		l.logger.InfoContext(ctx, msg, attributes...)
	}
}
