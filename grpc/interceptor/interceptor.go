package interceptor

import (
	"io"
	"os"

	"github.com/Azure/aks-middleware/grpc/client/mdforward"
	"github.com/Azure/aks-middleware/grpc/common"
	"github.com/Azure/aks-middleware/grpc/common/autologger"
	"github.com/Azure/aks-middleware/grpc/server/ctxlogger"
	"github.com/Azure/aks-middleware/grpc/server/requestid"
	"github.com/Azure/aks-middleware/grpc/server/responseheader"
	httpcommon "github.com/Azure/aks-middleware/http/common"

	log "log/slog"
	"strings"

	"github.com/bufbuild/protovalidate-go"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	protovalidate_middleware "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/protovalidate"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/retry"
	"google.golang.org/grpc"
)

type ClientInterceptorLogOptions struct {
	Logger     *log.Logger
	APIOutput  io.Writer
	Attributes []log.Attr
}

type ServerInterceptorLogOptions struct {
	Logger        *log.Logger
	APIOutput     io.Writer
	CtxOutput     io.Writer
	APIAttributes []log.Attr
	CtxAttributes []log.Attr
}

func GetClientInterceptorLogOptions(logger *log.Logger, attrs []log.Attr) ClientInterceptorLogOptions {
	return ClientInterceptorLogOptions{
		Logger:     logger,
		APIOutput:  os.Stdout,
		Attributes: attrs,
	}
}

func GetServerInterceptorLogOptions(logger *log.Logger, attrs []log.Attr) ServerInterceptorLogOptions {
	return ServerInterceptorLogOptions{
		Logger:        logger,
		APIOutput:     os.Stdout,
		CtxOutput:     os.Stdout,
		APIAttributes: attrs,
		CtxAttributes: attrs,
	}
}

func DefaultClientInterceptors(options ClientInterceptorLogOptions) []grpc.UnaryClientInterceptor {
	var apiHandler log.Handler

	apiHandlerOptions := &log.HandlerOptions{
		ReplaceAttr: func(groups []string, a log.Attr) log.Attr {
			a.Key = strings.TrimPrefix(a.Key, "grpc.")
			a.Key = strings.ReplaceAll(a.Key, ".", "_")
			return a
		},
	}

	if _, ok := options.Logger.Handler().(*log.JSONHandler); ok {
		apiHandler = log.NewJSONHandler(options.APIOutput, apiHandlerOptions)
	} else {
		apiHandler = log.NewTextHandler(options.APIOutput, apiHandlerOptions)
	}

	apiHandler = apiHandler.WithAttrs(options.Attributes)

	apiRequestLogger := log.New(apiHandler).With("source", "ApiRequestLog")
	return []grpc.UnaryClientInterceptor{
		retry.UnaryClientInterceptor(common.GetRetryOptions()...),
		mdforward.UnaryClientInterceptor(),
		logging.UnaryClientInterceptor(
			autologger.InterceptorLogger(apiRequestLogger),
			logging.WithLogOnEvents(logging.FinishCall),
			logging.WithLevels(logging.DefaultServerCodeToLevel),
		),
	}
}

func DefaultServerInterceptors(options ServerInterceptorLogOptions) []grpc.UnaryServerInterceptor {
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

	if _, ok := options.Logger.Handler().(*log.JSONHandler); ok {
		apiHandler = log.NewJSONHandler(options.APIOutput, apiHandlerOptions)
		ctxHandler = log.NewJSONHandler(options.CtxOutput, ctxHandlerOptions)
	} else {
		apiHandler = log.NewTextHandler(options.APIOutput, apiHandlerOptions)
		ctxHandler = log.NewTextHandler(options.CtxOutput, ctxHandlerOptions)
	}

	apiHandler = apiHandler.WithAttrs(options.APIAttributes)
	ctxHandler = ctxHandler.WithAttrs(options.CtxAttributes)

	apiRequestLogger := log.New(apiHandler).With("source", "ApiRequestLog")
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
			autologger.InterceptorLogger(apiRequestLogger),
			logging.WithLogOnEvents(logging.FinishCall),
			logging.WithFieldsFromContext(common.GetFields),
		),
		responseheader.UnaryServerInterceptor(httpcommon.MetadataToHeader),
		recovery.UnaryServerInterceptor(common.GetRecoveryOpts()...),
	}
}
