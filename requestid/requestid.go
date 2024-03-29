package requestid

import (
	"context"

	"crypto/rand"
	"encoding/base64"
	"io"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// Derived from https://github.com/goadesign/goa/blob/v3/grpc/middleware/requestid.go#L31

const (
	// RequestIDMetadataKey is the key in the gRPC
	// metadata.
	RequestIDMetadataKey = "x-request-id"
	RequestIDLogKey      = "request-id"
)

// UnaryServerInterceptor returns a server interceptor
// that add a request ID to the incoming metadata if there is none.
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler) (resp any, err error) {
		// log.Print("requestid ctx: ", ctx)
		ctx = generateRequestID(ctx)
		return handler(ctx, req)
	}
}

func generateRequestID(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		md = metadata.MD{}
	}
	if vals := md.Get(RequestIDMetadataKey); len(vals) > 0 {
		return ctx
	}
	md.Set(RequestIDMetadataKey, shortID())
	return metadata.NewIncomingContext(ctx, md)

}

func shortID() string {
	b := make([]byte, 6)
	io.ReadFull(rand.Reader, b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func GetRequestID(ctx context.Context) string {
	requestID := ""
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return requestID
	}
	if vals := md.Get(RequestIDMetadataKey); len(vals) > 0 {
		return vals[0]
	}
	return requestID
}
