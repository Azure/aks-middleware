package mdforward

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// UnaryClientInterceptor forwards the MD if there is no outgoing MD.
// It is only used in servers who make calls to dependencies on behalf an incoming request.
// This function propagates the MD information from the incoming requests to the server's dependencies.
// This function is not useful for a pure client program who doesn't have a incoming request.
func UnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		callOpts ...grpc.CallOption,
	) error {
		md, ok := metadata.FromIncomingContext(ctx)
		if ok {
			if _, ok := metadata.FromOutgoingContext(ctx); !ok {
				ctx = metadata.NewOutgoingContext(ctx, md)
			}
		}
		return invoker(ctx, method, req, reply, cc, callOpts...)
	}
}
