package logging

import (
	"context"
	"log/slog"
	"net/http"
	"reflect"
	"time"

	"github.com/gorilla/mux"
	"google.golang.org/grpc/metadata"
)

type initFunc func(w http.ResponseWriter, r *http.Request) map[string]interface{}
type loggingFunc func(w http.ResponseWriter, r *http.Request, attrs map[string]interface{}) map[string]interface{}
type AttributeManager struct {
	AttributeInitializer initFunc    // sets keys for custom attributes at the beginning of ServeHTTP()
	AttributeAssigner    loggingFunc // assigns values for custom attributes after request has completed
}

// TODO (Tom): Add a logger wrapper in its own package
// https://medium.com/@ansujain/building-a-logger-wrapper-in-go-with-support-for-multiple-logging-libraries-48092b826bee
// more info about http handler here: https://pkg.go.dev/net/http#Handler

// If source is empty, it will be set to "ApiRequestLog"
// If ANY fields in customAttributeAssigner are empty, or the struct itself is empty, default initializer and assigner will be set
func NewLogging(logger *slog.Logger, source string, customAttributeAssigner AttributeManager) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return &loggingMiddleware{
			next:                next,
			now:                 time.Now,
			logger:              *logger,
			source:              source,
			customAttributeInfo: customAttributeAssigner,
		}
	}
}

// enforcing that loggingMiddleware implements the http.Handler interface to ensure safety at compile time
var _ http.Handler = &loggingMiddleware{}

type loggingMiddleware struct {
	next                http.Handler
	now                 func() time.Time
	logger              slog.Logger
	source              string
	customAttributeInfo AttributeManager
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
	// If any fields in CustomAttributes are nil, do not call them to avoid errors
	setDefaults := attributeFunctionsNotSet(&l.customAttributeInfo)

	if setDefaults {
		setDefaultInitializerAndAssigner(&l.customAttributeInfo, l.source)
	}

	extraAttributes := (l.customAttributeInfo.AttributeInitializer)(w, r)

	customWriter := &responseWriter{ResponseWriter: w}
	startTime := l.now()
	ctx := r.Context()

	l.LogRequestStart(ctx, r, "RequestStart", extraAttributes)
	l.next.ServeHTTP(customWriter, r.WithContext(ctx))
	endTime := l.now()

	latency := endTime.Sub(startTime)
	updatedAttrs := (l.customAttributeInfo.AttributeAssigner)(customWriter, r, extraAttributes)

	l.LogRequestEnd(ctx, r, "RequestEnd", customWriter.statusCode, latency, updatedAttrs)
	l.LogRequestEnd(ctx, r, "finished call", customWriter.statusCode, latency, updatedAttrs)
}

func (l *loggingMiddleware) BuildLoggingAttributes(ctx context.Context, r *http.Request, extra ...interface{}) []interface{} {
	setSourceIfEmpty(&l.source)
	return BuildAttributes(ctx, l.source, r, extra...)
}

func BuildAttributes(ctx context.Context, source string, r *http.Request, extra ...interface{}) []interface{} {
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
	for _, e := range extra {
		flattened := flattenAttributes(e)
		attributes = append(attributes, flattened...)
	}

	attributes = append(attributes, "headers", headers)
	return attributes
}

func (l *loggingMiddleware) LogRequestStart(ctx context.Context, r *http.Request, msg string, extraAttributes map[string]interface{}) {
	attributes := l.BuildLoggingAttributes(ctx, r, extraAttributes)
	l.logger.InfoContext(ctx, msg, attributes...)
}

func (l *loggingMiddleware) LogRequestEnd(ctx context.Context, r *http.Request, msg string, statusCode int, duration time.Duration, extraAttributes map[string]interface{}) {
	attributes := l.BuildLoggingAttributes(ctx, r, "code", statusCode, "time_ms", duration.Milliseconds(), extraAttributes)
	l.logger.InfoContext(ctx, msg, attributes...)
}

// Returns true if extra attributes should be logged, false otherwise
func attributeFunctionsNotSet(attrStruct *AttributeManager) bool {
	if attrStruct == nil || attrStruct.AttributeInitializer == nil || attrStruct.AttributeAssigner == nil {
		return true
	} else {
		return false
	}
}

// Sets a default Initializer and Assigner function for Attribute Manager. Default attributes will be set in BuildAttributes()
func setDefaultInitializerAndAssigner(attributeManager *AttributeManager, source string) {
	defaultInitializer := func(w http.ResponseWriter, r *http.Request) map[string]interface{} {
		setSourceIfEmpty(&source)
		return nil
	}
	attributeManager.AttributeInitializer = defaultInitializer

	defaultAssigner := func(w http.ResponseWriter, r *http.Request, attrs map[string]interface{}) map[string]interface{} {
		return make(map[string]interface{}) // returning empty map because BuildAttributes sets default attributes regardless of default or user-defined assigner
	}
	if attributeManager.AttributeAssigner == nil {
		attributeManager.AttributeAssigner = defaultAssigner
	}
}

// Adds map k:v pairs as separate entires in []interface{} list for logging
func flattenAttributes(v interface{}) []interface{} {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Map {
		return []interface{}{v}
	}

	m, ok := v.(map[string]interface{})
	if !ok {
		return []interface{}{v}
	}

	attrList := make([]interface{}, 0, len(m)*2)
	for key, value := range m {
		attrList = append(attrList, key, value)
	}

	return attrList
}

// sets default source "ApiRequestLog"
func setSourceIfEmpty(source *string) {
	if len(*source) == 0 {
		*source = "ApiRequestLog"
	}
}

// default attributes that are set
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
