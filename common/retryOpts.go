package common

import (
	"time"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/retry"
	"google.golang.org/grpc/codes"
)

func GetRetryOptions() []retry.CallOption {
	retryOpts := []retry.CallOption{
		retry.WithBackoff(retry.BackoffExponential(100 * time.Millisecond)),
		retry.WithCodes(codes.Aborted, codes.Unavailable),
		retry.WithMax(3),
	}

	return retryOpts
}
