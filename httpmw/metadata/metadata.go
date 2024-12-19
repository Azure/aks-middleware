package metadata

import (
	"context"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc/metadata"
)

// Helper function to extract HTTP headers and put them into metadata.
func extractMetadata(headersToMetadata map[string]string, req *http.Request) metadata.MD {
	md := metadata.Pairs()
	for headerName, metadataKey := range headersToMetadata {
		if value := req.Header.Get(headerName); value != "" {
			md.Append(metadataKey, value)
		}
	}
	return md
}

// Helper function to select the metadata key and returns its corresponding HTTP header key.
func matchOutgoingHeader(allowedMetadataKeys map[string]string, s string) (string, bool) {
	if header, ok := allowedMetadataKeys[s]; ok {
		return header, true
	}
	return "", false
}

// NewMetadataMiddleware returns an array of ServeMuxOptions that can be used to convert incoming HTTP headers to gRPC metadata and vice versa.
func NewMetadataMiddleware(allowedMetadataKeys, headersToMetadata map[string]string) []runtime.ServeMuxOption {
	return []runtime.ServeMuxOption{
		runtime.WithMetadata(func(ctx context.Context, req *http.Request) metadata.MD {
			return extractMetadata(headersToMetadata, req)
		}),
		runtime.WithOutgoingHeaderMatcher(func(s string) (string, bool) {
			return matchOutgoingHeader(allowedMetadataKeys, s)
		}),
	}
}
