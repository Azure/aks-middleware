package responseheader_test

import (
	"context"
	"net"

	"github.com/Azure/aks-middleware/grpc/server/responseheader"
	pb "github.com/Azure/aks-middleware/test/api/v1"
	"github.com/Azure/aks-middleware/test/server"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

var _ = Describe("ResponseHeader Interceptor Integration", func() {
	var (
		grpcServer    *grpc.Server
		lis           net.Listener
		testSrv       *server.TestServer
		clientConn    *grpc.ClientConn
		greeterClient pb.MyGreeterClient
	)

	BeforeEach(func() {
		var err error
		lis, err = net.Listen("tcp", "localhost:0") // Listen on a random available port
		Expect(err).ToNot(HaveOccurred())

		metadataToHeader := map[string]string{
			"custom-header":      "X-Custom-Header",
			"another-header":     "X-Another-Header",
			"multi-value-header": "X-Multi-Value-Header",
			"empty-metadata-key": "",
		}
		grpcServer = grpc.NewServer(
			grpc.UnaryInterceptor(responseheader.UnaryServerInterceptor(metadataToHeader)),
		)
		testSrv = &server.TestServer{}
		pb.RegisterMyGreeterServer(grpcServer, testSrv)

		go func() {
			err := grpcServer.Serve(lis)
			Expect(err).NotTo(HaveOccurred())
		}()

		conn, err := grpc.Dial(lis.Addr().String(), grpc.WithInsecure())
		Expect(err).ToNot(HaveOccurred())
		clientConn = conn
		greeterClient = pb.NewMyGreeterClient(conn)
	})

	AfterEach(func() {
		clientConn.Close()
		grpcServer.Stop()
		lis.Close()
	})

	It("should propagate allowed metadata to response headers", func() {
		md := metadata.Pairs(
			"custom-header", "CustomValue",
			"another-header", "AnotherValue",
			"unallowed-key", "should-not-be-propagated",
		)
		ctx := metadata.NewOutgoingContext(context.Background(), md)

		var header metadata.MD
		resp, err := greeterClient.SayHello(ctx, &pb.HelloRequest{
			Name: "IntegrationTest",
			Age:  28,
		}, grpc.Header(&header)) // gRPC call option to capture response headers
		Expect(err).ToNot(HaveOccurred())
		Expect(resp.Message).To(Equal("Hello IntegrationTest"))

		// Verify allowed metadata
		Expect(header["custom-header"]).To(ContainElement("CustomValue"))
		Expect(header["another-header"]).To(ContainElement("AnotherValue"))

		// Verify unallowed metadata is not present
		Expect(header["unallowed-key"]).To(BeNil())

	})
})
