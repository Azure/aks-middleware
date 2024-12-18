package metadata

import (
	"context"
	"net/http"

	"github.com/Azure/aks-middleware/common"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc/metadata"
)

// NewMetadataMiddleware returns ServeMux options for header and metadata conversion.
func NewMetadataMiddleware() []runtime.ServeMuxOption {
	return []runtime.ServeMuxOption{
		runtime.WithMetadata(func(ctx context.Context, req *http.Request) metadata.MD {
			md := metadata.Pairs()
			if correlationID := req.Header.Get(common.RequestCorrelationIDHeader); correlationID != "" {
				md.Append(common.CorrelationIDKey, correlationID)
			}
			if operationID := req.Header.Get(common.RequestAcsOperationIDHeader); operationID != "" {
				md.Append(common.OperationIDKey, operationID)
			}
			if clientRequestID := req.Header.Get(common.RequestARMClientRequestIDHeader); clientRequestID != "" {
				md.Append(common.ARMClientRequestIDKey, clientRequestID)
			}
			return md
		}),
		runtime.WithOutgoingHeaderMatcher(func(s string) (string, bool) {
			allowedHeaders := map[string]string{
				common.OperationIDKey:        common.RequestAcsOperationIDHeader,
				common.ARMClientRequestIDKey: common.RequestARMClientRequestIDHeader,
			}
			if header, ok := allowedHeaders[s]; ok {
				return header, true
			}
			return "", false
		}),
	}
}
