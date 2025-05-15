package contextlogger

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	"log/slog"

	"github.com/Azure/aks-middleware/http/common"
	opreq "github.com/Azure/aks-middleware/http/server/operationrequest"
	"github.com/Azure/aks-middleware/http/server/requestid"
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

	BeforeEach(func() {
		logBuf = new(bytes.Buffer)
		logger := slog.New(slog.NewJSONHandler(logBuf, nil))

		router = mux.NewRouter()

		// Global middleware on the root router:
		// Add request ID interceptor so that every request gets a generated request ID.
		router.Use(requestid.NewRequestIDMiddleware())
		// Attach global context logging interceptor so that even routes without subrouter-specific
		// context interceptor get a default logger in the context.
		router.Use(New(*logger, nil))

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

		// Create a sub-router for API routes that require operation request injection.
		subRouter := router.PathPrefix("/subscriptions").Subrouter()

		// The following interceptor adds operation request details into the request's context.
		// If no context log interceptor is used on the subrouter (or if global context log interceptor is used only),
		// then logging will not include these  operation request details, since the top level interceptors gets executed first.
		// For the context log interceptor to get extra info from other interceptors, the other interceptors must execute before
		//
		// The operation request interceptor must come before the context logging interceptor
		// on the subrouter. This ensures that the operation request details are present in the context when
		// the context logging interceptor builds its attributes

		// TODO (tomabraham): Register a top level operation request interceptor that can dynamcally extract
		// relevant informaiton for all routes
		subRouter.Use(opreq.NewOperationRequest("test-region", defaultOpts))
		// Then add logging interceptor to capture op request details from the context.
		subRouter.Use(New(*logger, func(ctx context.Context, r *http.Request, w ResponseRecord) map[string]interface{} {
			op := opreq.OperationRequestFromContext(ctx)
			if op == nil {
				return nil
			}
			return opreq.FilteredOperationRequestMap(op, []string{
				"TargetURI", "HttpMethod", "AcceptedLanguage", "APIVersion", "Region",
				"SubscriptionID", "ResourceGroup", "ResourceName", "CorrelationID", "OperationID", "MyCustomHeader",
			})
		},
		))

		routePattern := fmt.Sprintf("/{%s}/resourceGroups/{%s}/providers/{%s}/{%s}/{%s}/default",
			common.SubscriptionIDKey, common.ResourceGroupKey, common.ResourceProviderKey, common.ResourceTypeKey, common.ResourceNameKey)

		subRouter.HandleFunc(routePattern, func(w http.ResponseWriter, r *http.Request) {
			l := GetLogger(r.Context())
			if l != nil {
				l.Info("integrated log message")
			}
			w.WriteHeader(http.StatusOK)
		}).Methods(http.MethodPost)

		// The /health route is attached directly to the root router.
		// Since the root router already has the global context logging interceptor,
		// it will log default attributes. However, it will not have any operation request data,
		// because those are only added by the operation request interceptor used on the subrouter.
		router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			l := GetLogger(r.Context())
			if l != nil {
				l.Info("health check log message")
			}
			w.WriteHeader(http.StatusOK)
		}).Methods(http.MethodGet)

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
		// Expect default ctx log fields to be present
		Expect(outStr).To(ContainSubstring("time"))
		Expect(outStr).To(ContainSubstring("level"))
		Expect(outStr).To(ContainSubstring("request-id"))
		Expect(outStr).To(ContainSubstring("method"))
		// Expect log to not contain op req fields that were not specified
		Expect(outStr).ToNot(ContainSubstring("RouteName"))
		Expect(outStr).ToNot(ContainSubstring("ResourceType"))
		Expect(outStr).ToNot(ContainSubstring("AdditionalHeader"))
	})

	It("should log default attributes for health check route in integration test", func() {
		url := server.URL + "/health"
		req, err := http.NewRequest(http.MethodGet, url, nil)
		Expect(err).NotTo(HaveOccurred())

		resp, err := http.DefaultClient.Do(req)
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()
		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		logOutput, err := io.ReadAll(logBuf)
		Expect(err).NotTo(HaveOccurred())
		outStr := string(logOutput)
		Expect(outStr).To(ContainSubstring("time"))
		Expect(outStr).To(ContainSubstring("level"))
		Expect(outStr).To(ContainSubstring("request"))
		Expect(outStr).To(ContainSubstring("method"))
		Expect(outStr).To(ContainSubstring("CtxLog"))
		// should not contain any operation request details
		Expect(outStr).ToNot(ContainSubstring("SubscriptionID"))
		Expect(outStr).ToNot(ContainSubstring("ResourceGroup"))
		Expect(outStr).ToNot(ContainSubstring("ResourceName"))
	})
})
