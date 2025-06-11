package contextlogger

import (
	"context"
	"encoding/json"
	log "log/slog"
	"net/http"
	"os"

	opreq "github.com/Azure/aks-middleware/http/server/operationrequest"
	"github.com/gorilla/mux"
	"google.golang.org/grpc/metadata"
)

type ExtractFunction func(ctx context.Context, r *http.Request) map[string]interface{}

const (
	ctxLogSource = "CtxLog"
)

type loggerKeyType int

const (
	loggerKey loggerKeyType = iota
)

// DefaultExtractor extracts operation request fields from the context.
// It returns the filtered map containing only the specified keys.
func DefaultExtractor(ctx context.Context, r *http.Request) map[string]interface{} {
	op := opreq.OperationRequestFromContext(ctx)
	if op == nil {
		return nil
	}
	return opreq.FilteredOperationRequestMap(op, []string{
		"TargetURI", "HttpMethod", "AcceptedLanguage", "APIVersion", "Region",
		"SubscriptionID", "ResourceGroup", "ResourceName", "CorrelationID", "OperationID",
	})
}

// New creates a context logging middleware.
// Parameters
//
//	logger:                  A slog.Logger instance used for logging. Any static attributes added to this logger before passing it in will be preserved
//	extractFunction:         ExtractFunction extracts information from the ctx and/or the request and put it in the logger
func New(logger log.Logger, extractFunction ExtractFunction) mux.MiddlewareFunc {
	if extractFunction == nil {
		extractFunction = DefaultExtractor
	}
	return func(next http.Handler) http.Handler {
		return &contextLogMiddleware{
			next:            next,
			logger:          logger,
			extractFunction: extractFunction,
		}
	}
}

// Enforce that contextLogMiddleware implements http.Handler.
var _ http.Handler = &contextLogMiddleware{}

type contextLogMiddleware struct {
	next            http.Handler
	logger          log.Logger
	extractFunction ExtractFunction
}

func (m *contextLogMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	attributes := BuildAttributes(ctx, r, m.extractFunction)

	contextLogger := m.logger.With(attributes...)
	ctx = context.WithValue(ctx, loggerKey, contextLogger)
	r = r.WithContext(ctx)

	m.next.ServeHTTP(w, r)
}

func BuildAttributes(ctx context.Context, r *http.Request, extractFunc func(ctx context.Context, r *http.Request) map[string]interface{}) []interface{} {
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

	// Use the extract function to get additional attributes.
	if extractFunc != nil {
		extractedAttrs := extractFunc(ctx, r)
		for k, v := range extractedAttrs {
			logAttrs[k] = v
		}
	}

	attrBytes, err := json.Marshal(logAttrs)
	templogger := log.New(log.NewJSONHandler(os.Stdout, nil))
	if err != nil {
		templogger.With("source", ctxLogSource).Error("error building attributes for additional attributes", "error", err)
	}
	attributesStr := string(attrBytes)

	headerBytes, err := json.Marshal(headers)
	if err != nil {
		templogger.With("source", ctxLogSource).Error("error marshaling headers", "error", err)
	}
	headersStr := string(headerBytes)

	// Include metadata headers as part of the attributes.
	attributes = append(attributes, "log", attributesStr)
	// grab desired headers from the request (based on extraction function passed to request ID middleware)
	attributes = append(attributes, "headers", headersStr)
	return attributes
}

func defaultCtxLogAttributes(r *http.Request) []interface{} {
	return []interface{}{
		"request", r.URL.Path,
		"method", r.Method,
		"source", ctxLogSource,
	}
}

func GetLogger(ctx context.Context) *log.Logger {
	logger := log.Default().With("src", "self gen, not available in ctx")
	if ctx == nil {
		return logger
	}
	if ctxlogger, ok := ctx.Value(loggerKey).(*log.Logger); ok {
		return ctxlogger
	}
	return logger
}

func WithLogger(ctx context.Context, logger *log.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}
