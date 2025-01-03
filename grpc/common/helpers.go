package common

import (
	"context"

	httpcommon "github.com/Azure/aks-middleware/http/common"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"google.golang.org/grpc/metadata"
)

// GetFields returns a logging.Fields object with the request ID and headers
func GetFields(ctx context.Context) logging.Fields {
	headers := getMetadata(ctx)
	return logging.Fields{
		"headers", headers,
	}
}

func getMetadata(ctx context.Context) map[string]string {
	headersFromMD := make(map[string]string)
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return headersFromMD
	}
	for _, key := range []string{
		httpcommon.RequestIDMetadataHeader,
		httpcommon.CorrelationIDKey,
		httpcommon.OperationIDKey,
		httpcommon.ARMClientRequestIDKey,
	} {
		if vals := md.Get(key); len(vals) > 0 {
			headersFromMD[key] = vals[0]
		}
	}
	return headersFromMD
}
