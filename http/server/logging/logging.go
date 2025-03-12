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
type CustomAttributes struct {
	AttributeInitializer *initFunc    // sets keys for custom attributes at the beginning of ServeHTTP()
	AttributeAssigner    *loggingFunc // assigns values for custom attributes after request has completed
}

// TODO (Tom): Add a logger wrapper in its own package
// https://medium.com/@ansujain/building-a-logger-wrapper-in-go-with-support-for-multiple-logging-libraries-48092b826bee
// more info about http handler here: https://pkg.go.dev/net/http#Handler

// If source is empty, it will be set to "ApiRequestLog"
// If ANY fields in customAttributeAssigner are empty, or the struct itself is empty, extra attributes will not be assigned
func NewLogging(logger *slog.Logger, source string, customAttributeAssigner CustomAttributes) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return &loggingMiddleware{
			next:                    next,
			now:                     time.Now,
			logger:                  *logger,
			source:                  source,
			customAttributeAssigner: customAttributeAssigner,
		}
	}
}

// enforcing that loggingMiddleware implements the http.Handler interface to ensure safety at compile time
var _ http.Handler = &loggingMiddleware{}

type loggingMiddleware struct {
	next                    http.Handler
	now                     func() time.Time
	logger                  slog.Logger
	source                  string
	customAttributeAssigner CustomAttributes
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
	addExtraAttributes := validateCustomAttributes(&l.customAttributeAssigner)
	var extraAttributes map[string]interface{}
	if addExtraAttributes {
		extraAttributes = (*l.customAttributeAssigner.AttributeInitializer)(w, r) // we don't want to return error from this, which is dangerous. Need error checking to make sure that each custom attribute is being taken care of by func..?
	}

	customWriter := &responseWriter{ResponseWriter: w}
	startTime := l.now()
	ctx := r.Context()

	l.LogRequestStart(ctx, r, "RequestStart", extraAttributes)
	l.next.ServeHTTP(customWriter, r.WithContext(ctx))
	endTime := l.now()

	latency := endTime.Sub(startTime)

	var updatedAttrs map[string]interface{}
	if addExtraAttributes {
		updatedAttrs = (*l.customAttributeAssigner.AttributeAssigner)(w, r, extraAttributes)
	}

	l.LogRequestEnd(ctx, r, "RequestEnd", customWriter.statusCode, latency, updatedAttrs)
	l.LogRequestEnd(ctx, r, "finished call", customWriter.statusCode, latency, updatedAttrs)
}

func (l *loggingMiddleware) BuildLoggingAttributes(ctx context.Context, r *http.Request, extra ...interface{}) []interface{} {
	if len(l.source) == 0 {
		l.source = "ApiRequestLog"
	}

	return BuildAttributes(ctx, l.source, r, extra...)
}

func BuildAttributes(ctx context.Context, source string, r *http.Request, extra ...interface{}) []interface{} {
	if len(source) == 0 {
		source = "ApiRequestLog"
	}

	md, ok := metadata.FromIncomingContext(ctx)
	attributes := []interface{}{
		"source", source,
		"protocol", "HTTP",
		"method_type", "unary",
		"component", "server",
		"method", r.Method,
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
func validateCustomAttributes(attrStruct *CustomAttributes) bool {
	if attrStruct == nil {
		return false
	} else if attrStruct.AttributeInitializer == nil {
		return false
	} else if attrStruct.AttributeAssigner == nil {
		return false
	} else {
		return true
	}
}

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
		attrList = append(attrList, key)
		attrList = append(attrList, value)
	}

	return attrList

}
