package common

import (
	"context"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"google.golang.org/grpc/metadata"
)

func GetFields(ctx context.Context) logging.Fields {
	headers := GetMetadata(ctx)
	requestID := headers[RequestIDMetadataKey]
	// Remove the main request ID from headers map since it's logged separately
	delete(headers, RequestIDMetadataKey)
	return logging.Fields{
		RequestIDLogKey, requestID,
		"headers", headers,
	}
}

func GetMetadata(ctx context.Context) map[string]string {
	headersFromMD := make(map[string]string)
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return headersFromMD
	}
	for _, key := range []string{
		RequestIDMetadataKey,
		CorrelationIDKey,
		OperationIDKey,
		ARMClientRequestIDKey,
	} {
		if vals := md.Get(key); len(vals) > 0 {
			headersFromMD[key] = vals[0]
		}
	}
	return headersFromMD
}
