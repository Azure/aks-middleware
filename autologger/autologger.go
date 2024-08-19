package autologger

// This package is one implementation to control what to be logged for each API automatically.
// It provides customization function to go-grpc-middleware logging interceptor.

import (
	"context"
	"fmt"

	"github.com/Azure/aks-middleware/requestid"
	"github.com/Azure/aks-middleware/unifiedlogger"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
)

func InterceptorLogger(logger *unifiedlogger.LoggerWrapper) logging.Logger {
	return logging.LoggerFunc(func(ctx context.Context, lvl logging.Level, msg string, fields ...any) {
		f := make(map[string]any, len(fields)/2)
		i := logging.Fields(fields).Iterator()
		for i.Next() {
			k, v := i.At()
			f[k] = v
		}

		switch lvl {
		case logging.LevelDebug:
			logger.Debug(msg, f)
		case logging.LevelInfo:
			logger.Info(msg, f)
		case logging.LevelWarn:
			logger.Warn(msg, f)
		case logging.LevelError:
			logger.Error(msg, f)
		default:
			panic(fmt.Sprintf("unknown level %v", lvl))
		}
	})
}

// Add request-id to API autologger.
func GetFields(ctx context.Context) logging.Fields {
	return logging.Fields{
		requestid.RequestIDLogKey, requestid.GetRequestID(ctx),
	}
}
