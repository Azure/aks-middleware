package contextlogger

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	opreq "github.com/Azure/aks-middleware/http/server/operationrequest"
	"github.com/gorilla/mux"
	"google.golang.org/grpc/metadata"

	"github.com/Azure/aks-middleware/http/common"
)

type loggerKeyType string

const (
	ctxLogSource               = "CtxLog"
	ctxLoggerKey loggerKeyType = "CtxLogKey"
)

// NewContextLogMiddleware creates a context logging middleware.
// Parameters
//
//	logger:          A slog.Logger instance used for logging.
//	extraAttributes: A map containing additional key/value pairs that will be merged
//	                  with the default attributes.
//	opFields:        A slice of strings indicating operation-specific fields to be included
//	                 in the logging context.
func NewContextLogMiddleware(logger slog.Logger, extraAttributes map[string]interface{}, opFields []string) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return &contextLogMiddleware{
			next:            next,
			logger:          logger,
			extraAttributes: extraAttributes,
			opFields:        opFields,
		}
	}
}

// Enforce that contextLogMiddleware implements http.Handler.
var _ http.Handler = &contextLogMiddleware{}

type contextLogMiddleware struct {
	next            http.Handler
	logger          slog.Logger
	extraAttributes map[string]interface{}
	opFields        []string
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

func (m *contextLogMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	customWriter := &ResponseRecord{ResponseWriter: w}

	ctx := r.Context()
	attributes := BuildAttributes(ctx, r, m.extraAttributes, m.opFields)
	contextLogger := m.logger.With(attributes...)
	ctx = context.WithValue(ctx, ctxLoggerKey, contextLogger)
	r = r.WithContext(ctx)

	m.next.ServeHTTP(customWriter, r)
}

func BuildAttributes(ctx context.Context, r *http.Request, extra map[string]interface{}, opFields []string) []interface{} {
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

    // If an OperationRequest exists in the context, use it.
    if op := opreq.OperationRequestFromContext(ctx); op != nil {
        flatMap := opreq.FlattenOperationRequest(op)
        filtered := make(map[string]interface{})
        // For each requested field (from opFields)
        for _, reqField := range opFields {
            // Look in the top-level flattened map.
            for key, val := range flatMap {
                if strings.EqualFold(key, reqField) {
                    filtered[key] = val
                }
            }
            // If the key is in the Extras sub-map, include it.
            if extrasVal, exists := flatMap["Extras"]; exists {
                if extrasMap, ok := extrasVal.(map[string]interface{}); ok {
                    for extraKey, extraVal := range extrasMap {
                        if strings.EqualFold(extraKey, reqField) {
                            filtered[extraKey] = extraVal
                        }
                    }
                }
            }
        }
        // Merge the filtered fields into log attributes.
        for k, v := range filtered {
            logAttrs[k] = v
        }
    }

    attributes = append(attributes, "log", logAttrs)
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
