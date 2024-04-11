package interceptor

import (
	"io"

	"github.com/Azure/aks-middleware/autologger"
	"github.com/Azure/aks-middleware/ctxlogger"
	"github.com/Azure/aks-middleware/mdforward"
	"github.com/Azure/aks-middleware/requestid"

	log "log/slog"
	"strings"

	"github.com/bufbuild/protovalidate-go"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	protovalidate_middleware "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/protovalidate"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/retry"
	"google.golang.org/grpc"
)

type ClientInterceptorOptions struct {
	Logger     *log.Logger
	APIOutput  io.Writer
	Attributes []log.Attr
}

type ServerInterceptorOptions struct {
	Logger     *log.Logger
	APIOutput  io.Writer
	CtxOutput  io.Writer 
	APIAttributes []log.Attr
	CtxAttributes []log.Attr
}

func DefaultClientInterceptors(clientInterceptorOptions ClientInterceptorOptions) []grpc.UnaryClientInterceptor {
	var apiHandler log.Handler

	apiHandlerOptions := &log.HandlerOptions{
		ReplaceAttr: func(groups []string, a log.Attr) log.Attr {
			a.Key = strings.TrimPrefix(a.Key, "grpc.")
			a.Key = strings.ReplaceAll(a.Key, ".", "_")
			return a
		},
	}

	if _, ok := clientInterceptorOptions.Logger.Handler().(*log.JSONHandler); ok {
		apiHandler = log.NewJSONHandler(clientInterceptorOptions.APIOutput, apiHandlerOptions)
	} else {
		apiHandler = log.NewTextHandler(clientInterceptorOptions.APIOutput, apiHandlerOptions)
	}

	apiHandler = apiHandler.WithAttrs(clientInterceptorOptions.Attributes)

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

func DefaultServerInterceptors(serverInterceptorOptions ServerInterceptorOptions) []grpc.UnaryServerInterceptor {
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

	if _, ok := serverInterceptorOptions.Logger.Handler().(*log.JSONHandler); ok {
		apiHandler = log.NewJSONHandler(serverInterceptorOptions.APIOutput, apiHandlerOptions)
		ctxHandler = log.NewJSONHandler(serverInterceptorOptions.CtxOutput, ctxHandlerOptions)
	} else {
		apiHandler = log.NewTextHandler(serverInterceptorOptions.APIOutput, apiHandlerOptions)
		ctxHandler = log.NewTextHandler(serverInterceptorOptions.CtxOutput, ctxHandlerOptions)
	}

	apiHandler = apiHandler.WithAttrs(serverInterceptorOptions.APIAttributes)
	ctxHandler = ctxHandler.WithAttrs(serverInterceptorOptions.CtxAttributes)

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
