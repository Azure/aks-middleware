package operationrequest

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/Azure/aks-middleware/http/common"
	"github.com/gorilla/mux"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type TestExtras struct {
	MyCustomHeader string `json:"myCustomHeader"`
}

var _ = Describe("OperationRequest Context Examination Integration", func() {
	var (
		router   *mux.Router
		server   *httptest.Server
		validURL string
	)

	// Define a customizer that extracts an extra header.
	extrasCustomizer := OperationRequestCustomizerFunc[TestExtras](func(e *TestExtras, headers http.Header, vars map[string]string) error {
		if v := headers.Get("X-Custom-Extra"); v != "" {
			e.MyCustomHeader = v
		}
		return nil
	})

	defaultOpts := OperationRequestOptions[TestExtras]{
		Extras:     TestExtras{},
		Customizer: extrasCustomizer,
	}

	BeforeEach(func() {
		// Setup the mux router.
		router = mux.NewRouter()
		routePattern := "/subscriptions/{subscriptionId}/resourceGroups/{resourceGroup}/providers/{resourceProvider}/{resourceType}/{resourceName}/default"
		validURL = "/subscriptions/sub3/resourceGroups/rg3/providers/Microsoft.Test/providerType1/resourceName1/default?api-version=2021-12-01"

		// Create the final handler that extracts the OperationRequest from context and returns it as JSON.
		finalHandler := func(w http.ResponseWriter, r *http.Request) {
			op := OperationRequestFromContext[TestExtras](r.Context())
			if op == nil {
				http.Error(w, "missing operation request", http.StatusInternalServerError)
				return
			}
			b, err := json.Marshal(op)
			if err != nil {
				http.Error(w, "marshal error", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write(b)
		}

		router.Use(NewOperationRequest("region-test", defaultOpts))

		router.Methods("POST").
			Path(routePattern).
			Name("exampleRoute").
			HandlerFunc(finalHandler)

		server = httptest.NewServer(router)
	})

	AfterEach(func() {
		server.Close()
	})

	It("should attach a fully populated OperationRequest to the context", func() {
		payload := "integration test payload"
		req, err := http.NewRequest(http.MethodPost, server.URL+validURL, strings.NewReader(payload))
		Expect(err).NotTo(HaveOccurred())
		req.Header.Set(common.RequestCorrelationIDHeader, "corr-test-context")
		req.Header.Set(common.RequestAcceptLanguageHeader, "EN-GB")
		// Do not provide an OperationID header so that one is auto-generated.
		req.Header.Set("X-Custom-Extra", "customValue")

		// The full pipeline routes the request through the middleware which sets URL variables,
		// attaches the current route, and builds the OperationRequest.
		resp, err := http.DefaultClient.Do(req)
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()
		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		data, err := io.ReadAll(resp.Body)
		Expect(err).NotTo(HaveOccurred())

		var op BaseOperationRequest[TestExtras]
		err = json.Unmarshal(data, &op)
		Expect(err).NotTo(HaveOccurred())

		Expect(op.APIVersion).To(Equal("2021-12-01"))
		Expect(op.SubscriptionID).To(Equal("sub3"))
		Expect(op.ResourceGroup).To(Equal("rg3"))
		Expect(op.CorrelationID).To(Equal("corr-test-context"))
		Expect(op.HttpMethod).To(Equal(http.MethodPost))
		Expect(op.TargetURI).To(ContainSubstring("api-version=2021-12-01"))
		Expect(op.OperationID).NotTo(BeEmpty())
		Expect(op.RouteName).To(Equal("exampleRoute"))
		Expect(op.Region).To(Equal("region-test"))
		Expect(op.Body).To(Equal([]byte(payload)))
		Expect(op.Extras.MyCustomHeader).To(Equal("customValue"))
	})

	It("should return an error when api-version is missing", func() {
		payload := "payload without api-version"
		// Create a URL without the required query parameter.
		errorURL := "/subscriptions/sub3/resourceGroups/rg3/providers/Microsoft.Test/providerType1/resourceName1/default"
		req, err := http.NewRequest(http.MethodPost, server.URL+errorURL, strings.NewReader(payload))
		Expect(err).NotTo(HaveOccurred())
		req.Header.Set(common.RequestCorrelationIDHeader, "corr-error")
		req.Header.Set(common.RequestAcceptLanguageHeader, "EN-GB")
		req.Header.Set("X-Custom-Extra", "customValue")

		resp, err := http.DefaultClient.Do(req)
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()

		Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))

		data, err := io.ReadAll(resp.Body)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(ContainSubstring("no api-version in URI's parameters"))
	})
})
