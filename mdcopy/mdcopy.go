package mdcopy

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// UnaryServerInterceptor returns a server interceptor
// that copies request metadata into response metadata.
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		// Copy the metadata
		if err := copyMetadata(ctx); err != nil {
			fmt.Println("Failed to copy metadata:", err)
		}
		// Proceed with the handler
		return handler(ctx, req)
	}
}

func copyMetadata(ctx context.Context) error {
	// Extract incoming metadata from the context
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		fmt.Println("No incoming metadata found")
		return nil
	}
	// Set the outgoing header metadata
	if err := grpc.SetHeader(ctx, md); err != nil {
		return err
	}
	return nil
}
