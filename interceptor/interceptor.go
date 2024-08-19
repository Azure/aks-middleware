package interceptor

import (
	"io"
	"os"

	"github.com/Azure/aks-middleware/autologger"
	"github.com/Azure/aks-middleware/ctxlogger"
	"github.com/Azure/aks-middleware/mdforward"
	"github.com/Azure/aks-middleware/requestid"
	"github.com/Azure/aks-middleware/unifiedlogger"

	"strings"

	"github.com/bufbuild/protovalidate-go"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	protovalidate_middleware "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/protovalidate"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/retry"
	"google.golang.org/grpc"
)

type ClientInterceptorLogOptions struct {
	Logger     *unifiedlogger.LoggerWrapper
	APIOutput  io.Writer
	Attributes []unifiedlogger.LoggerWrapper
}

type ServerInterceptorLogOptions struct {
	Logger        *unifiedlogger.LoggerWrapper
	APIOutput     io.Writer
	CtxOutput     io.Writer
	APIAttributes []unifiedlogger.LoggerWrapper
	CtxAttributes []unifiedlogger.LoggerWrapper
}

func GetClientInterceptorLogOptions(logger *unifiedlogger.LoggerWrapper, attrs []unifiedlogger.LoggerWrapper) ClientInterceptorLogOptions {
	return ClientInterceptorLogOptions{
		Logger:     logger,
		APIOutput:  os.Stdout,
		Attributes: attrs,
	}
}

func GetServerInterceptorLogOptions(logger *unifiedlogger.LoggerWrapper, attrs []unifiedlogger.LoggerWrapper) ServerInterceptorLogOptions {
	return ServerInterceptorLogOptions{
		Logger:        logger,
		APIOutput:     os.Stdout,
		CtxOutput:     os.Stdout,
			APIAttributes: attrs,
		CtxAttributes: attrs,
	}
}

func DefaultClientInterceptors(options ClientInterceptorLogOptions) []grpc.UnaryClientInterceptor {
	var apiHandler unifiedlogger.LoggerWrapper

	apiHandlerOptions := &unifiedlogger.LoggerWrapper{
		ReplaceAttr: func(groups []string, a unifiedlogger.LoggerWrapper) unifiedlogger.LoggerWrapper {
			a.Key = strings.TrimPrefix(a.Key, "grpc.")
			a.Key = strings.ReplaceAll(a.Key, ".", "_")
			return a
		},
	}

	if _, ok := options.Logger.Handler().(*unifiedlogger.LoggerWrapper); ok {
		apiHandler = unifiedlogger.NewJSONHandler(options.APIOutput, apiHandlerOptions)
	} else {
		apiHandler = unifiedlogger.NewTextHandler(options.APIOutput, apiHandlerOptions)
	}

	apiHandler = apiHandler.WithAttrs(options.Attributes)

	apiRequestLogger := unifiedlogger.New(apiHandler).With("source", "ApiRequestLog")
	return []grpc.UnaryClientInterceptor{
		retry.UnaryClientInterceptor(GetRetryOptions()...),
		mdforward.UnaryClientInterceptor(),
		logging.UnaryClientInterceptor(
			autologger.InterceptorLogger(apiRequestLogger),
			logging.WithLogOnEvents(logging.FinishCall),
			logging.WithLevels(logging.DefaultServerCodeToLevel),
			logging.WithFieldsFromContext(autologger.GetFields),
		),
	}
}

func DefaultServerInterceptors(options ServerInterceptorLogOptions) []grpc.UnaryServerInterceptor {
	// The first registerred interceptor will be called first.
	// Need to register requestid first to add request-id.
	// Then the logger can get the request-id.
	var apiHandler unifiedlogger.LoggerWrapper
	var ctxHandler unifiedlogger.LoggerWrapper

	apiHandlerOptions := &unifiedlogger.LoggerWrapper{
		ReplaceAttr: func(groups []string, a unifiedlogger.LoggerWrapper) unifiedlogger.LoggerWrapper {
			a.Key = strings.TrimPrefix(a.Key, "grpc.")
			a.Key = strings.ReplaceAll(a.Key, ".", "_")
			return a
		},
	}
	ctxHandlerOptions := &unifiedlogger.LoggerWrapper{
		AddSource: true,
		ReplaceAttr: func(groups []string, a unifiedlogger.LoggerWrapper) unifiedlogger.LoggerWrapper {
			if a.Key == unifiedlogger.SourceKey {
				// Needed to add to prevent "CtxLog" key from being changed as well
				switch value := a.Value.Any().(type) {
				case *unifiedlogger.Source:
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

	if _, ok := options.Logger.Handler().(*unifiedlogger.LoggerWrapper); ok {
		apiHandler = unifiedlogger.NewJSONHandler(options.APIOutput, apiHandlerOptions)
		ctxHandler = unifiedlogger.NewJSONHandler(options.CtxOutput, ctxHandlerOptions)
	} else {
		apiHandler = unifiedlogger.NewTextHandler(options.APIOutput, apiHandlerOptions)
		ctxHandler = unifiedlogger.NewTextHandler(options.CtxOutput, ctxHandlerOptions)
	}

	apiHandler = apiHandler.WithAttrs(options.APIAttributes)
	ctxHandler = ctxHandler.WithAttrs(options.CtxAttributes)

	apiRequestLogger := unifiedlogger.New(apiHandler).With("source", "ApiRequestLog")
	appCtxlogger := unifiedlogger.New(ctxHandler).With("source", "CtxLog")
	validator, err := protovalidate.New()
	if err != nil {
		panic(err)
	}
	return []grpc.UnaryServerInterceptor{
		protovalidate_middleware.UnaryServerInterceptor(validator),
		requestid.UnaryServerInterceptor(),
		ctxlogger.UnaryServerInterceptor(appCtxlogger, nil),
		logging.UnaryServerInterceptor(
			autologger.InterceptorLogger(apiRequestLogger),
			logging.WithLogOnEvents(logging.FinishCall),
			logging.WithFieldsFromContext(autologger.GetFields),
		),
		recovery.UnaryServerInterceptor(GetRecoveryOpts()...),
	}
}
