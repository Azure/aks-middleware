package autologger

// This package is one implementation to control what to be logged for each API automatically.
// It provides customization function to go-grpc-middleware logging interceptor.

import (
	"context"
	"fmt"

	"github.com/Azure/aks-middleware/requestid"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/sirupsen/logrus"
)

func InterceptorLogger(logger logrus.FieldLogger) logging.Logger {
	return logging.LoggerFunc(func(ctx context.Context, lvl logging.Level, msg string, fields ...any) {
		// fmt.Println("ctx: ", ctx)
		// fmt.Printf("fields: %v\n", fields)
		f := make(map[string]any, len(fields)/2)
		i := logging.Fields(fields).Iterator()
		for i.Next() {
			k, v := i.At()
			f[k] = v
			// fmt.Printf("k %v, v %v\n", k, v)
		}
		l := logger.WithFields(f)
		// fmt.Println(lvl, msg)
		// l.Info("blah")

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

// Add request-id to API autologger.
func GetFields(ctx context.Context) logging.Fields {
	return logging.Fields{
		requestid.RequestIDLogKey, requestid.GetRequestID(ctx),
	}
}
