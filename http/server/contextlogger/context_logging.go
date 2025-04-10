package contextlogger

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gorilla/mux"
	"google.golang.org/grpc/metadata"
)

type loggerKeyType string

type Options struct {
	ExtraAttributes map[string]interface{}
	ExtractFunc     func(ctx context.Context, r *http.Request) map[string]interface{}
}

const (
	ctxLogSource               = "CtxLog"
	ctxLoggerKey loggerKeyType = "CtxLogKey"
)

// New creates a context logging middleware.
// Parameters
//
//	logger:          A slog.Logger instance used for logging.
//	options:         An Options instance containing extra attributes and an extract function.
func New(logger slog.Logger, options Options) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return &contextLogMiddleware{
			next:    next,
			logger:  logger,
			options: options,
		}
	}
}

// Enforce that contextLogMiddleware implements http.Handler.
var _ http.Handler = &contextLogMiddleware{}

type contextLogMiddleware struct {
	next    http.Handler
	logger  slog.Logger
	options Options
}

func (m *contextLogMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	attributes := BuildAttributes(ctx, r, m.options.ExtraAttributes, m.options.ExtractFunc)
	contextLogger := m.logger.With(attributes...)
	ctx = context.WithValue(ctx, ctxLoggerKey, contextLogger)
	r = r.WithContext(ctx)

	m.next.ServeHTTP(w, r)
}

func BuildAttributes(ctx context.Context, r *http.Request, extra map[string]interface{}, extractFunc func(ctx context.Context, r *http.Request) map[string]interface{}) []interface{} {
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
	logAttrs := make(map[string]interface{})

	// Merge any caller-supplied extra attributes.
	for k, v := range extra {
		logAttrs[k] = v
	}

	// Use the extract function to get additional attributes.
	if extractFunc != nil {
		extractedAttrs := extractFunc(ctx, r)
		for k, v := range extractedAttrs {
			logAttrs[k] = v
		}
	}

	// Include metadata headers as part of the attributes.
	attributes = append(attributes, "log", logAttrs)
	// grab desired headers from the request (based on extraction function passed to request ID middleware)
	attributes = append(attributes, "headers", headers)
	return attributes
}

func defaultCtxLogAttributes(r *http.Request) []interface{} {
	requestPath := r.URL.Path
	return []interface{}{
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
