package contextlogger

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"google.golang.org/grpc/metadata"

	"github.com/Azure/aks-middleware/http/common"
)

type AttributeInitializerFunc func(w *ResponseRecord, r *http.Request) map[string]interface{}
type AttributeAssignerFunc func(w *ResponseRecord, r *http.Request, attrs map[string]interface{}) map[string]interface{}

type AttributeManager struct {
	AttributeInitializer AttributeInitializerFunc // sets keys for custom attributes at the beginning of ServeHTTP()
	AttributeAssigner    AttributeAssignerFunc    // assigns values for custom attributes after request has completed
}

const (
	ctxLogSource = "CtxLog"
)

// If source is empty, it will be set to "CtxLog"
// If a field in the attributeAssigner are empty, or the struct itself is empty, default functions will be set
func NewLogging(logger slog.Logger, source string, attributeManager AttributeManager) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return &customAttributeLoggingMiddleware{
			next:             next,
			now:              time.Now,
			logger:           logger,
			source:           source,
			attributeManager: &attributeManager,
		}
	}
}

// Enforcing that loggingMiddleware implements the http.Handler interface to ensure safety at compile time
var _ http.Handler = &customAttributeLoggingMiddleware{}

type customAttributeLoggingMiddleware struct {
	next             http.Handler
	now              func() time.Time
	logger           slog.Logger
	source           string
	attributeManager *AttributeManager
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

func (l *customAttributeLoggingMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Println("source: ", l.source)
	if len(l.source) == 0 {
		l.source = ctxLogSource
	}

	addextraattributes := false
	extraAttributes := make(map[string]interface{})

	customWriter := &ResponseRecord{ResponseWriter: w}

	if l.attributeManager != nil || l.source == ctxLogSource {
		addextraattributes = true
		setInitializerAndAssignerIfNil(l.attributeManager)
		extraAttributes = (l.attributeManager.AttributeInitializer)(customWriter, r)
	}

	startTime := l.now()
	ctx := r.Context()

	l.LogRequestStart(ctx, r, "RequestStart", extraAttributes)

	defer func() {
		if err := recover(); err != nil {
			ctx = context.WithValue(ctx, "panic", err)
			r = r.WithContext(ctx)
		}
	}()

	l.next.ServeHTTP(customWriter, r.WithContext(ctx))
	endTime := l.now()

	latency := endTime.Sub(startTime)

	var updatedAttrs map[string]interface{}
	if addextraattributes {
		updatedAttrs = (l.attributeManager.AttributeAssigner)(customWriter, r, extraAttributes)
	} else {
		updatedAttrs = extraAttributes
	}

	updatedAttrs["code"] = customWriter.statusCode
	updatedAttrs["time_ms"] = latency.Milliseconds()

	l.LogRequestEnd(ctx, r, "RequestEnd", updatedAttrs)
	l.LogRequestEnd(ctx, r, "finished call", updatedAttrs)
}

func BuildAttributes(ctx context.Context, source string, r *http.Request, extra map[string]interface{}) []interface{} {
	md, ok := metadata.FromIncomingContext(ctx)

	headers := make(map[string]string)
	if ok {
		for key, values := range md {
			if len(values) > 0 {
				headers[key] = values[0]
			}
		}
	}

	var attributes []interface{}
	if source == ctxLogSource { // do not flatten attributes, they will be added to "log" column
		attributes = defaultCtxLogAttributes(r)
		attributes = append(attributes, "log", extra)
	} else {
		flattened := flattenAttributes(extra)
		attributes = append(attributes, "source", source)
		attributes = append(attributes, flattened...)
	}

	attributes = append(attributes, "headers", headers)
	return attributes
}

func (l *customAttributeLoggingMiddleware) LogRequestStart(ctx context.Context, r *http.Request, msg string, extraAttributes map[string]interface{}) {
	attributes := BuildAttributes(ctx, l.source, r, extraAttributes)
	l.logger.InfoContext(ctx, msg, attributes...)
}

func (l *customAttributeLoggingMiddleware) LogRequestEnd(ctx context.Context, r *http.Request, msg string, extraAttributes map[string]interface{}) {
	attributes := BuildAttributes(ctx, l.source, r, extraAttributes)
	l.logger.InfoContext(ctx, msg, attributes...)
}

// Sets the initializer and/or assigner to a default function if nil
func setInitializerAndAssignerIfNil(attrManager *AttributeManager) {
	if attrManager.AttributeInitializer == nil {
		attrManager.AttributeInitializer = NewAttributeInitializer()
	}

	if attrManager.AttributeAssigner == nil {
		attrManager.AttributeAssigner = NewAttributeAssigner()
	}
}

// Returns default attribute initializer
func NewAttributeInitializer() AttributeInitializerFunc {
	return func(w *ResponseRecord, r *http.Request) map[string]interface{} {
		return make(map[string]interface{})
	}
}

// Returns default attribute assigner
func NewAttributeAssigner() AttributeAssignerFunc {
	return func(w *ResponseRecord, r *http.Request, attrs map[string]interface{}) map[string]interface{} {
		return make(map[string]interface{}) // returning empty map because BuildAttributes sets default attributes regardless of default or user-defined assigner
	}
}

// Adds map k:v pairs as separate entires in []interface{} list for logging
func flattenAttributes(m map[string]interface{}) []interface{} {
	attrList := make([]interface{}, 0, len(m)*2)
	for key, value := range m {
		attrList = append(attrList, key, value)
	}

	return attrList
}

func defaultCtxLogAttributes(r *http.Request) []interface{} {
	var level slog.Level
	var request string
	if r.Context().Err() != nil {
		level = slog.LevelError
	} else {
		level = slog.LevelInfo
		request = r.URL.Path
	}

	return []interface{}{
		"source", ctxLogSource,
		"time", time.Now(),
		"level", level,
		"request_id", r.Header.Get(common.RequestARMClientRequestIDHeader),
		"request", request,
		"method", r.Method,
	}
}
