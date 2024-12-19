package metadata

import (
	"context"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc/metadata"
)

func NewMetadataMiddleware(allowedHeaders, headersToMetadata map[string]string) []runtime.ServeMuxOption {
	return []runtime.ServeMuxOption{
		runtime.WithMetadata(func(ctx context.Context, req *http.Request) metadata.MD {
			md := metadata.Pairs()
			for headerName, metadataKey := range headersToMetadata {
				if value := req.Header.Get(headerName); value != "" {
					md.Append(metadataKey, value)
				}
			}
			return md
		}),
		runtime.WithOutgoingHeaderMatcher(func(s string) (string, bool) {
			if header, ok := allowedHeaders[s]; ok {
				return header, true
			}
			return "", false
		}),
	}
}
