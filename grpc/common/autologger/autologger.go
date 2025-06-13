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

func InterceptorLogger(logger *log.Logger) logging.Logger {
	return logging.LoggerFunc(func(ctx context.Context, lvl logging.Level, msg string, fields ...any) {
		// fmt.Println("ctx: ", ctx)
		// fmt.Printf("fields: %v\n", fields)
		f := make(map[string]any, len(fields)/2)
		l := logger

		// add default header fields such as operationId, correlationId, etc
		l = l.With(common.GetFields(ctx)...)

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
