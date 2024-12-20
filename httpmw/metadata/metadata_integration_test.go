package metadata

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"

	"github.com/Azure/aks-middleware/responseheader"
	pb "github.com/Azure/aks-middleware/test/api/v1"
	testServer "github.com/Azure/aks-middleware/test/server"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var _ = Describe("Metadata Integration", func() {
	var (
		mux         *runtime.ServeMux
		testHTTPSrv *httptest.Server
		grpcServer  *grpc.Server
		testSrv     *testServer.TestServer
		lis         net.Listener
	)

	BeforeEach(func() {
		metadataToHeader := map[string]string{
			"custom-header":      "X-Custom-Header",
			"another-header":     "X-Another-Header",
			"multi-value-header": "X-Multi-Value-Header",
			"empty-metadata-key": "",
		}
		headerToMetadata := map[string]string{
			"X-Custom-Header":      "custom-header",
			"X-Another-Header":     "another-header",
			"X-Multi-Value-Header": "multi-value-header",
			"X-Empty-Metadata":     "",
		}

		responseInterceptor := responseheader.UnaryServerInterceptor(metadataToHeader)
		grpcServer = grpc.NewServer(grpc.UnaryInterceptor(responseInterceptor))
		testSrv = &testServer.TestServer{}
		pb.RegisterMyGreeterServer(grpcServer, testSrv)

		var err error
		lis, err = net.Listen("tcp", "localhost:0")
		Expect(err).NotTo(HaveOccurred())

		go func() {
			err := grpcServer.Serve(lis)
			Expect(err).NotTo(HaveOccurred())
		}()

		mux = runtime.NewServeMux(NewMetadataMiddleware(headerToMetadata, metadataToHeader)...)

		// Register the gRPC-Gateway handler to forward HTTP to gRPC
		err = pb.RegisterMyGreeterHandlerFromEndpoint(context.Background(), mux, lis.Addr().String(), []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		})
		Expect(err).NotTo(HaveOccurred())

		// Create a test HTTP server
		testHTTPSrv = httptest.NewServer(mux)
	})

	AfterEach(func() {
		testHTTPSrv.Close()
		grpcServer.Stop()
		lis.Close()
	})

	Describe("extractMetadata", func() {
		It("should handle multiple headers and extract only mapped ones", func() {
			req, err := http.NewRequest("POST", testHTTPSrv.URL+"/v1/hello", nil)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("X-Custom-Header", "value1")
			req.Header.Set("X-Another-Header", "value2")
			req.Header.Set("X-Irrelevant-Header", "value3")

			resp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(testSrv.ReceivedMetadata["custom-header"]).To(ContainElement("value1"))
			Expect(testSrv.ReceivedMetadata["another-header"]).To(ContainElement("value2"))
			Expect(testSrv.ReceivedMetadata).NotTo(HaveKey("irrelevant-header"))
		})

	})

	Describe("matchOutgoingHeader", func() {
		It("should match allowed headers and set outgoing HTTP headers", func() {
			req, err := http.NewRequest("POST", testHTTPSrv.URL+"/v1/hello", nil)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("X-Custom-Header", "value")
			req.Header.Set("X-Disallowed-Header", "value")

			resp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			// Verify the outgoing header
			Expect(resp.Header.Get("X-Custom-Header")).To(Equal("value"))
			// Disallowed headers should not be present in the response
			Expect(resp.Header.Get("X-Disallowed-Header")).To(Equal(""))
		})
	})
})
