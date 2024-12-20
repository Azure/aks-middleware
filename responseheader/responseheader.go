package responseheader

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// UnaryServerInterceptor returns a server interceptor
// that copies selected request metadata into response metadata.
func UnaryServerInterceptor(allowedMetadataKeys map[string]string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if err := copyMetadata(ctx, allowedMetadataKeys); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

func copyMetadata(ctx context.Context, allowedMetadataKeys map[string]string) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil
	}
	// Filter and set the allowed metadata as response headers
	filteredMD := metadata.New(nil)
	for key := range allowedMetadataKeys {
		if values, exists := md[key]; exists {
			filteredMD.Set(key, values...)
		}
	}

	if err := grpc.SetHeader(ctx, filteredMD); err != nil {
		return err
	}
	return nil
}
