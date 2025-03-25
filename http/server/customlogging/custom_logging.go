package customlogging

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"google.golang.org/grpc/metadata"
)

type AttributeInitializerFunc func(w *ResponseRecord, r *http.Request) map[string]interface{}
type AttributeAssignerFunc func(w *ResponseRecord, r *http.Request, attrs map[string]interface{}) map[string]interface{}

type AttributeManager struct {
	AttributeInitializer AttributeInitializerFunc // sets keys for custom attributes at the beginning of ServeHTTP()
	AttributeAssigner    AttributeAssignerFunc    // assigns values for custom attributes after request has completed
}

const (
	apiRequestLogSource = "ApiRequestLog"
)

// If source is empty, it will be set to "ApiRequestLog"
// If a field in the attributeAssigner are empty, or the struct itself is empty, default functions will be set
func NewLogging(logger *slog.Logger, source string, attributeManager AttributeManager) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return &customAttributeLoggingMiddleware{
			next:              next,
			now:               time.Now,
			logger:            *logger,
			source:            source,
			attributemManager: &attributeManager,
		}
	}
}

// Enforcing that loggingMiddleware implements the http.Handler interface to ensure safety at compile time
var _ http.Handler = &customAttributeLoggingMiddleware{}

type customAttributeLoggingMiddleware struct {
	next              http.Handler
	now               func() time.Time
	logger            slog.Logger
	source            string
	attributemManager *AttributeManager
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
	setSourceIfEmpty(&l.source)
	addextraattributes := false
	extraAttributes := make(map[string]interface{})

	customWriter := &ResponseRecord{ResponseWriter: w}

	if l.attributemManager != nil || l.source == apiRequestLogSource {
		addextraattributes = true
		setInitializerAndAssignerIfNil(l.attributemManager)
		extraAttributes = (l.attributemManager.AttributeInitializer)(customWriter, r)
	}

	startTime := l.now()
	ctx := r.Context()

	l.LogRequestStart(ctx, r, "RequestStart", extraAttributes)
	l.next.ServeHTTP(customWriter, r.WithContext(ctx))
	endTime := l.now()

	latency := endTime.Sub(startTime)

	var updatedAttrs map[string]interface{}
	if addextraattributes {
		updatedAttrs = (l.attributemManager.AttributeAssigner)(customWriter, r, extraAttributes)
	} else {
		updatedAttrs = extraAttributes
	}

	updatedAttrs["code"] = customWriter.statusCode
	updatedAttrs["time_ms"] = latency.Milliseconds()

	l.LogRequestEnd(ctx, r, "RequestEnd", updatedAttrs)
	l.LogRequestEnd(ctx, r, "finished call", updatedAttrs)
}

func (l *customAttributeLoggingMiddleware) BuildLoggingAttributes(ctx context.Context, r *http.Request, extra map[string]interface{}) []interface{} {
	return BuildAttributes(ctx, l.source, r, extra)
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

	attributes := flattenAttributes(defaultAttributes(source, r))
	flattened := flattenAttributes(extra)
	attributes = append(attributes, flattened...)

	attributes = append(attributes, "headers", headers)
	return attributes
}

func (l *customAttributeLoggingMiddleware) LogRequestStart(ctx context.Context, r *http.Request, msg string, extraAttributes map[string]interface{}) {
	attributes := l.BuildLoggingAttributes(ctx, r, extraAttributes)
	l.logger.InfoContext(ctx, msg, attributes...)
}

func (l *customAttributeLoggingMiddleware) LogRequestEnd(ctx context.Context, r *http.Request, msg string, extraAttributes map[string]interface{}) {
	attributes := l.BuildLoggingAttributes(ctx, r, extraAttributes)
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

// Sets default source "ApiRequestLog"
func setSourceIfEmpty(source *string) {
	if len(*source) == 0 {
		*source = apiRequestLogSource
	}
}

// Default attributes set for any request
func defaultAttributes(source string, r *http.Request) map[string]interface{} {
	return map[string]interface{}{
		"source":      &source,
		"protocol":    "HTTP",
		"method_type": "unary",
		"component":   "server",
		"method":      r.Method,
		"service":     r.Host,
		"url":         r.URL.String(),
	}
}
