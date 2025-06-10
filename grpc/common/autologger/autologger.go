package autologger

// This package is one implementation to control what to be logged for each API automatically.
// It provides customization function to go-grpc-middleware logging interceptor.

import (
	"context"
	"fmt"

	log "log/slog"

	opreq "github.com/Azure/aks-middleware/http/server/operationrequest"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
)

func InterceptorLogger(logger *log.Logger) logging.Logger {
	return logging.LoggerFunc(func(ctx context.Context, lvl logging.Level, msg string, fields ...any) {
		// fmt.Println("ctx: ", ctx)
		// fmt.Printf("fields: %v\n", fields)
		f := make(map[string]any, len(fields)/2)
		l := logger

		// Extract operation request from context if available
		if op := opreq.OperationRequestFromContext(ctx); op != nil {
			// Only add the IDs to the headers field, not as top-level attributes
			headers := make(map[string]string)
			if op.CorrelationID != "" {
				headers["correlation_id"] = op.CorrelationID
			}
			if op.OperationID != "" {
				headers["operation_id"] = op.OperationID
			}
			if len(headers) > 0 {
				l = l.With("headers", headers)
			}
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
