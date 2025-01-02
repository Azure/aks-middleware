package policy_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"

	log "log/slog"

	serviceHubPolicy "github.com/Azure/aks-middleware/httpclient-azuresdk/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("LoggingPolicy", func() {
	var (
		logger *log.Logger
		buf    bytes.Buffer
		server *ghttp.Server
	)

	BeforeEach(func() {
		logger = log.New(log.NewJSONHandler(&buf, nil))
		buf = *new(bytes.Buffer)
		server = ghttp.NewServer()
	})

	AfterEach(func() {
		server.Close()
	})

	Context("when the request is successful", func() {
		It("logs the request and response", func() {
			server.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest(http.MethodGet, "/"),
				ghttp.RespondWith(http.StatusOK, "Hello, world!"),
			))
			clientOptions := new(policy.ClientOptions)
			pipelineOptions := new(runtime.PipelineOptions)
			clientOptions.PerCallPolicies = append(clientOptions.PerCallPolicies, serviceHubPolicy.NewLoggingPolicy(*logger))
			pipeline := runtime.NewPipeline("", "", *pipelineOptions, clientOptions)
			req, err := runtime.NewRequest(context.Background(), http.MethodGet, server.URL())
			Expect(err).NotTo(HaveOccurred())

			resp, err := pipeline.Do(req)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			Expect(buf.String()).To(ContainSubstring("finished call"))
			Expect(buf.String()).To(ContainSubstring(fmt.Sprintf("\"code\":%d", http.StatusOK)))
			Expect(buf.String()).To(ContainSubstring(fmt.Sprintf("\"method\":\"%s", http.MethodGet)))
			Expect(buf.String()).To(ContainSubstring("\"time_ms\":"))
		})
	})
})
