package contextlogger

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"google.golang.org/grpc/metadata"

	"github.com/Azure/aks-middleware/http/common"
)

type loggerKeyType string

const (
	ctxLogSource               = "CtxLog"
	ctxLoggerKey loggerKeyType = "CtxLogKey"
)

// NewLogging creates a context logging middleware.
// The caller can supply a map of custom attributes in extraAttributes,
// which will be merged with the default attributes.
func NewLogging(logger slog.Logger, extraAttributes map[string]interface{}) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return &customAttributeLoggingMiddleware{
			next:            next,
			logger:          logger,
			extraAttributes: extraAttributes,
		}
	}
}

// Enforce that customAttributeLoggingMiddleware implements http.Handler.
var _ http.Handler = &customAttributeLoggingMiddleware{}

type customAttributeLoggingMiddleware struct {
	next            http.Handler
	logger          slog.Logger
	extraAttributes map[string]interface{}
}

type ResponseRecord struct {
	http.ResponseWriter
	statusCode int
}

func (r *ResponseRecord) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *ResponseRecord) Write(b []byte) (int, error) {
	if r.statusCode == 0 {
		r.statusCode = http.StatusOK
	}
	return r.ResponseWriter.Write(b)
}

func (r *ResponseRecord) StatusCode() int {
	return r.statusCode
}

func (m *customAttributeLoggingMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	customWriter := &ResponseRecord{ResponseWriter: w}

	ctx := r.Context()
	attributes := BuildAttributes(ctx, r, m.extraAttributes)
	contextLogger := m.logger.With(attributes...)
	ctx = context.WithValue(ctx, ctxLoggerKey, contextLogger)
	r = r.WithContext(ctx)

	m.next.ServeHTTP(customWriter, r)
}

// BuildAttributes creates a slice of key/value pairs used for logging.
func BuildAttributes(ctx context.Context, r *http.Request, extra map[string]interface{}) []interface{} {
	md, ok := metadata.FromIncomingContext(ctx)
	headers := make(map[string]string)
	if ok {
		for key, values := range md {
			if len(values) > 0 {
				headers[key] = values[0]
			}
		}
	}

	attributes := defaultCtxLogAttributes(r)
	if extra != nil {
		attributes = append(attributes, "log", extra)
	}

	attributes = append(attributes, "headers", headers)
	return attributes
}

func defaultCtxLogAttributes(r *http.Request) []interface{} {
	var level slog.Level
	requestPath := r.URL.Path
	if r.Context().Err() != nil {
		level = slog.LevelError
	} else {
		level = slog.LevelInfo
	}

	return []interface{}{
		"time", time.Now(),
		"level", level,
		"request_id", r.Header.Get(common.RequestARMClientRequestIDHeader),
		"request", requestPath,
		"method", r.Method,
		"source", ctxLogSource,
	}
}

// GetLogger returns the logger stored in the context.
// It will return nil if no logger was injected.
func GetLogger(ctx context.Context) *slog.Logger {
	logger, ok := ctx.Value(ctxLoggerKey).(*slog.Logger)
	if !ok {
		return nil
	}
	return logger
}
