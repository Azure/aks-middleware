package server

import (
	"context"

	"google.golang.org/grpc/metadata"

	pb "github.com/Azure/aks-middleware/test/api/v1"
)

// TestServer implements the MyGreeterServer interface and captures incoming metadata.
type TestServer struct {
	pb.UnimplementedMyGreeterServer
	// ReceivedMetadata stores the metadata extracted from the incoming gRPC context.
	ReceivedMetadata metadata.MD
}

// SayHello implements the SayHello RPC method.
// It captures the incoming metadata and returns a greeting message.
func (s *TestServer) SayHello(ctx context.Context, req *pb.HelloRequest) (*pb.HelloReply, error) {
	// Store the incoming metadata in the ReceivedMetadata field so we can test the WithMetadata middleware individually
	s.ReceivedMetadata, _ = metadata.FromIncomingContext(ctx)

	return &pb.HelloReply{Message: "Hello " + req.Name}, nil
}
