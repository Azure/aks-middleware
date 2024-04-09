package interceptor

import (
	"io"

	"github.com/Azure/aks-middleware/autologger"
	"github.com/Azure/aks-middleware/ctxlogger"
	"github.com/Azure/aks-middleware/mdforward"
	"github.com/Azure/aks-middleware/requestid"
	"go.goms.io/aks/rp/toolkit/aksbinversion"

	log "log/slog"
	"strings"

	"github.com/bufbuild/protovalidate-go"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	protovalidate_middleware "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/protovalidate"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/retry"
	"google.golang.org/grpc"
)

func DefaultClientInterceptors(logger log.Logger, apiOutput io.Writer) []grpc.UnaryClientInterceptor {
	var apiHandler log.Handler

	apiHandlerOptions := &log.HandlerOptions{
		ReplaceAttr: func(groups []string, a log.Attr) log.Attr {
			a.Key = strings.TrimPrefix(a.Key, "grpc.")
			a.Key = strings.ReplaceAll(a.Key, ".", "_")
			return a
		},
	}

	log.Info("FROM AKS BIN VERSION " + aksbinversion.GetVersion())

	// logger.SetOutput(apiOutput)

	// // Extract all fields from the record
	// var attrs []slog.Attr
	// record.Attrs(func(attr slog.Attr) bool {
	// 	attrs = append(attrs, attr)
	// 	return true // Continue iterating
	// })

	// // Now 'attrs' contains all the dynamically determined fields
	// for key, value := range attrs {
	// 	fmt.Printf("Field: %s, Value: %v\n", key, value)
	// }

	if _, ok := logger.Handler().(*log.JSONHandler); ok {
		apiHandler = log.NewJSONHandler(apiOutput, apiHandlerOptions)
		// apiHandler.WithAttrs(attrs)
	} else {
		apiHandler = log.NewTextHandler(apiOutput, apiHandlerOptions)
	}

	apiAutologger := log.New(apiHandler).With("source", "ApiAutoLog")
	return []grpc.UnaryClientInterceptor{
		retry.UnaryClientInterceptor(GetRetryOptions()...),
		mdforward.UnaryClientInterceptor(),
		logging.UnaryClientInterceptor(
			autologger.InterceptorLogger(apiAutologger),
			logging.WithLogOnEvents(logging.FinishCall),
			logging.WithLevels(logging.DefaultServerCodeToLevel),
			logging.WithFieldsFromContext(autologger.GetFields),
		),
	}
}

func DefaultServerInterceptors(logger log.Logger, apiOutput io.Writer, ctxOutput io.Writer) []grpc.UnaryServerInterceptor {
	// The first registerred interceptor will be called first.
	// Need to register requestid first to add request-id.
	// Then the logger can get the request-id.
	var apiHandler log.Handler
	var ctxHandler log.Handler

	apiHandlerOptions := &log.HandlerOptions{
		ReplaceAttr: func(groups []string, a log.Attr) log.Attr {
			a.Key = strings.TrimPrefix(a.Key, "grpc.")
			a.Key = strings.ReplaceAll(a.Key, ".", "_")
			return a
		},
	}
	ctxHandlerOptions := &log.HandlerOptions{
		AddSource: true,
		ReplaceAttr: func(groups []string, a log.Attr) log.Attr {
			if a.Key == log.SourceKey {
				// Needed to add to prevent "CtxLog" key from being changed as well
				switch value := a.Value.Any().(type) {
				case *log.Source:
					if strings.Contains(value.File, ".go") {
						a.Key = "location"
					}
				}
			}
			a.Key = strings.TrimPrefix(a.Key, "grpc.")
			a.Key = strings.ReplaceAll(a.Key, ".", "_")
			return a
		},
	}

	if _, ok := logger.Handler().(*log.JSONHandler); ok {
		apiHandler = log.NewJSONHandler(apiOutput, apiHandlerOptions)
		ctxHandler = log.NewJSONHandler(ctxOutput, ctxHandlerOptions)
	} else {
		apiHandler = log.NewTextHandler(apiOutput, apiHandlerOptions)
		ctxHandler = log.NewTextHandler(ctxOutput, ctxHandlerOptions)
	}

	apiAutologger := log.New(apiHandler).With("source", "ApiAutoLog")
	appCtxlogger := log.New(ctxHandler).With("source", "CtxLog")
	validator, err := protovalidate.New()
	if err != nil {
		panic(err)
	}
	return []grpc.UnaryServerInterceptor{
		protovalidate_middleware.UnaryServerInterceptor(validator),
		requestid.UnaryServerInterceptor(),
		ctxlogger.UnaryServerInterceptor(appCtxlogger, nil),
		logging.UnaryServerInterceptor(
			autologger.InterceptorLogger(apiAutologger),
			logging.WithLogOnEvents(logging.FinishCall),
			logging.WithFieldsFromContext(autologger.GetFields),
		),
		recovery.UnaryServerInterceptor(GetRecoveryOpts()...),
	}
}
