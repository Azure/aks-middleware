package autologger

// This package is one implementation to control what to be logged for each API automatically.
// It provides customization function to go-grpc-middleware logging interceptor.

import (
	"context"
	"fmt"

	log "log/slog"

	"github.com/Azure/aks-middleware/grpc/common"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
)

// HeadersFunc extracts headers from the context to be added to the logger.
// It should return a map[string]string containing header key-value pairs.
type HeadersFunc func(ctx context.Context) map[string]string

// InterceptorLogger creates a logging.Logger that automatically extracts Azure headers from the gRPC context.
// This is the default implementation that uses common.GetFields to extract standard Azure headers
// like correlation-id, operation-id, request-id, and arm-client-request-id from gRPC metadata.
func InterceptorLogger(logger *log.Logger) logging.Logger {
	return InterceptorLoggerWithHeadersFunc(logger, nil)
}

// InterceptorLoggerWithHeadersFunc creates a logging.Logger with custom header extraction logic.
// This provides flexibility to customize which headers are extracted from the context.
// Pass nil as headersFunc to use the default common.GetFields header extraction.
func InterceptorLoggerWithHeadersFunc(logger *log.Logger, headersFunc HeadersFunc) logging.Logger {
	return logging.LoggerFunc(func(ctx context.Context, lvl logging.Level, msg string, fields ...any) {
		f := make(map[string]any, len(fields)/2)
		l := logger

		// Add headers using custom function or default common.GetFields
		// when using custom function, only allow caller to update headers column
		if headersFunc != nil {
			headers := headersFunc(ctx)
			if len(headers) > 0 {
				l = l.With("headers", headers)
			}
		} else {
			// Use default common.GetFields
			l = l.With(common.GetFields(ctx)...)
		}

		// Process the fields from the interceptor
		i := logging.Fields(fields).Iterator()
		for i.Next() {
			k, v := i.At()
			f[k] = v
			l = l.With(k, v)
		}

		switch lvl {
		case logging.LevelDebug:
			l.Debug(msg)
		case logging.LevelInfo:
			l.Info(msg)
		case logging.LevelWarn:
			l.Warn(msg)
		case logging.LevelError:
			l.Error(msg)
		default:
			panic(fmt.Sprintf("unknown level %v", lvl))
		}
	})
}
