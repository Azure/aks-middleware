package interceptor

import (
	"github.com/Azure/aks-middleware/autologger"
	"github.com/Azure/aks-middleware/ctxlogger"
	"github.com/Azure/aks-middleware/mdforward"
	"github.com/Azure/aks-middleware/requestid"

	log "log/slog"

	"github.com/bufbuild/protovalidate-go"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	protovalidate_middleware "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/protovalidate"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/retry"
	"google.golang.org/grpc"
)

func DefaultClientInterceptors(logger log.Logger) []grpc.UnaryClientInterceptor {
	apiAutologger := logger.With("source", "ApiAutoLog")
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

func DefaultServerInterceptors(logger log.Logger) []grpc.UnaryServerInterceptor {
	// The first registerred interceptor will be called first.
	// Need to register requestid first to add request-id.
	// Then the logger can get the request-id.
	apiAutologger := logger.With("source", "ApiAutoLog")
	appCtxlogger := logger.With("source", "CtxLog")
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
