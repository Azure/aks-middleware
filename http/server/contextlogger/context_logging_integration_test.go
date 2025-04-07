package contextlogger

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	"log/slog"

	"github.com/Azure/aks-middleware/http/common"
	opreq "github.com/Azure/aks-middleware/http/server/operationrequest"
	"github.com/gorilla/mux"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("OperationRequest and ContextLogger Integration", func() {
	var (
		router *mux.Router
		server *httptest.Server
		logBuf *bytes.Buffer
	)

	// Define a customizer that extracts an extra header.
	extrasCustomizer := opreq.OperationRequestCustomizerFunc(func(e map[string]interface{}, headers http.Header, vars map[string]string) error {
		if v := headers.Get("X-Custom-Extra"); v != "" {
			e["MyCustomHeader"] = v
			e["AdditionalHeader"] = "extraValue"
		}
		return nil
	})

	defaultOpts := opreq.OperationRequestOptions{
		Extras:     make(map[string]interface{}),
		Customizer: extrasCustomizer,
	}

	BeforeEach(func() {
		router = mux.NewRouter()

		subRouter := router.PathPrefix("/subscriptions").Subrouter()
		subRouter.Use(opreq.NewOperationRequest("test-region", defaultOpts))

		logBuf = new(bytes.Buffer)
		logger := slog.New(slog.NewJSONHandler(logBuf, nil))

		// operation request fields that should be included in ctxlog attrs
		// caller can specify what fields to include to keep it generic
		opFields := []string{
			"TargetURI",
			"HttpMethod",
			"AcceptedLanguage",
			"APIVersion",
			"Region",
			"SubscriptionID",
			"ResourceGroup",
			"ResourceName",
			"CorrelationID",
			"OperationID",
			"MyCustomHeader",
		}
		subRouter.Use(NewContextLogMiddleware(*logger, nil, opFields))

		routePattern := fmt.Sprintf("/{%s}/resourceGroups/{%s}/providers/{%s}/{%s}/{%s}/default",
			common.SubscriptionIDKey, common.ResourceGroupKey, common.ResourceProviderKey, common.ResourceTypeKey, common.ResourceNameKey)

		subRouter.HandleFunc(routePattern, func(w http.ResponseWriter, r *http.Request) {
			op := opreq.OperationRequestFromContext(r.Context())
			if op == nil {
				http.Error(w, "missing operation request", http.StatusInternalServerError)
				return
			}
			l := GetLogger(r.Context())
			if l != nil {
				l.Info("integrated log message")
			}
			w.WriteHeader(http.StatusOK)
		}).Methods(http.MethodPost)

		server = httptest.NewServer(router)
	})

	AfterEach(func() {
		server.Close()
	})

	It("should log specified operation request details via context logger", func() {
		url := server.URL + "/subscriptions/sub123/resourceGroups/rg123/providers/Microsoft.Test/resourceType1/resourceName/default?api-version=2021-12-01"
		req, err := http.NewRequest(http.MethodPost, url, strings.NewReader("payload-data"))
		Expect(err).NotTo(HaveOccurred())
		req.Header.Set(common.RequestCorrelationIDHeader, "corr-test")
		req.Header.Set(common.RequestAcceptLanguageHeader, "en-us")
		req.Header.Set("X-Custom-Extra", "extraValue")

		resp, err := http.DefaultClient.Do(req)
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()
		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		logOutput, err := io.ReadAll(logBuf)
		Expect(err).NotTo(HaveOccurred())
		outStr := string(logOutput)
		// Expect the log to contain specified operation request details.
		Expect(outStr).To(ContainSubstring(`"SubscriptionID":"sub123"`))
		Expect(outStr).To(ContainSubstring(`"ResourceGroup":"rg123"`))
		Expect(outStr).To(ContainSubstring(`"ResourceName":"resourceName"`))
		Expect(outStr).To(ContainSubstring(`"APIVersion":"2021-12-01"`))
		Expect(outStr).To(ContainSubstring(`"CorrelationID":"corr-test"`))
		Expect(outStr).To(ContainSubstring(`"AcceptedLanguage":"en-us"`))
		Expect(outStr).To(ContainSubstring(`"MyCustomHeader":"extraValue"`))
		Expect(outStr).To(ContainSubstring("integrated log message"))
		// Expect log to not contain op req fields that were not specified
		Expect(outStr).ToNot(ContainSubstring("RouteName"))
		Expect(outStr).ToNot(ContainSubstring(`"ResourceType":"Microsoft.Test/resourceType1"`))
		Expect(outStr).ToNot(ContainSubstring("AdditionalHeader"))
	})
})
