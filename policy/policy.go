package policy

import (
	"net/http"
	"time"

	"github.com/Azure/aks-middleware/logging"
	"github.com/Azure/aks-middleware/unifiedlogger"
	armPolicy "github.com/Azure/azure-sdk-for-go/sdk/azcore/arm/policy"
	azcorePolicy "github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"google.golang.org/grpc/codes"
)

type LoggingPolicy struct {
	logger unifiedlogger.LoggerWrapper
}

func NewLoggingPolicy(logger unifiedlogger.LoggerWrapper) *LoggingPolicy {
	return &LoggingPolicy{logger: logger}
}

func (p *LoggingPolicy) Do(req *azcorePolicy.Request) (*http.Response, error) {
	startTime := time.Now()
	resp, err := req.Next()

	logging.LogRequest(logging.LogRequestParams{
		Logger:    &p.logger,
		StartTime: startTime,
		Request:   req.Raw(),
		Response:  resp,
		Error:     err,
	})
	return resp, err
}

func (p *LoggingPolicy) Clone() azcorePolicy.Policy {
	return &LoggingPolicy{logger: p.logger}
}

func GetDefaultArmClientOptions(logger *unifiedlogger.LoggerWrapper) *armPolicy.ClientOptions {
	logOptions := new(azcorePolicy.LogOptions)

	retryOptions := new(azcorePolicy.RetryOptions)
	retryOptions.MaxRetries = 5

	clientOptions := new(azcorePolicy.ClientOptions)
	clientOptions.Logging = *logOptions
	clientOptions.Retry = *retryOptions

	armClientOptions := new(armPolicy.ClientOptions)
	armClientOptions.ClientOptions = *clientOptions

	loggingPolicy := NewLoggingPolicy(*logger)

	armClientOptions.PerCallPolicies = append(armClientOptions.PerCallPolicies, loggingPolicy)

	return armClientOptions
}

// Based off of gRPC standard here: https://chromium.googlesource.com/external/github.com/grpc/grpc/+/refs/tags/v1.21.4-pre1/doc/statuscodes.md
func ConvertHTTPStatusToGRPCError(httpStatusCode int) codes.Code {
	var code codes.Code

	switch httpStatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusAccepted:
		code = codes.OK
	case http.StatusBadRequest:
		code = codes.InvalidArgument
	case http.StatusGatewayTimeout:
		code = codes.DeadlineExceeded
	case http.StatusUnauthorized:
		code = codes.Unauthenticated
	case http.StatusForbidden:
		code = codes.PermissionDenied
	case http.StatusNotFound:
		code = codes.NotFound
	case http.StatusConflict:
		code = codes.Aborted
	case http.StatusTooManyRequests:
		code = codes.ResourceExhausted
	case http.StatusInternalServerError:
		code = codes.Internal
	case http.StatusNotImplemented:
		code = codes.Unimplemented
	case http.StatusServiceUnavailable:
		code = codes.Unavailable
	default:
		code = codes.Unknown
	}

	return code
}
