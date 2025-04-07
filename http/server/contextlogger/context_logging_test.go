package contextlogger

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"

	"github.com/Azure/aks-middleware/http/server/requestid"
	"github.com/gorilla/mux"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// routerConfig now only holds the extraAttributes map (source is not customizable).
type routerConfig struct {
	buf             *bytes.Buffer
	logger          *slog.Logger
	extraAttributes map[string]interface{}
}

const (
	subscriptionIDKey    = "subscriptionID"
	resourceGroupNameKey = "resourceGroupName"
	resultTypeKey        = "resultType"
	errorDetailsKey      = "errorDetails"

	defaultRouterName         = "default"
	onlyAssignerRouterName    = "only-assigner-set"
	onlyInitializerRouterName = "only-initializer-set"
	extraLoggingVariablesName = "extra-logging-variables"

	customTestKey   = "testKey"
	customTestValue = "testValue"
)

var _ = Describe("HttpmwWithCustomAttributeLogging", Ordered, func() {
	var (
		// customExtractor remains the same.
		customExtractor = func(r *http.Request) map[string]string {
			return map[string]string{
				string(requestid.CorrelationIDKey): r.Header.Get(requestid.RequestCorrelationIDHeader),
				string(requestid.OperationIDKey):   r.Header.Get(requestid.RequestAcsOperationIDHeader),
			}
		}

		// router configurations supply a simple extraAttributes map.
		testRoutersConfigurationMap = map[string]*routerConfig{
			defaultRouterName: {
				extraAttributes: nil,
			},
			onlyAssignerRouterName: {
				extraAttributes: map[string]interface{}{
					customTestKey: customTestValue,
				},
			},
			onlyInitializerRouterName: {
				extraAttributes: map[string]interface{}{
					customTestKey: customTestValue,
				},
			},
			extraLoggingVariablesName: {
				extraAttributes: map[string]interface{}{
					subscriptionIDKey:    "defaultSubIDvalue",
					resourceGroupNameKey: "defaultRGnamevalue",
					resultTypeKey:        2,
					errorDetailsKey:      "defaultErrorDetailsvalue",
				},
			},
		}

		routersMap = map[string]*mux.Router{}
	)

	// buildRouter creates a mux.Router, installs the requestid middleware,
	// the context logging middleware (using NewLogging without source customization),
	// and a test handler that retrieves the logger via GetLogger and logs a message.
	buildRouter := func(cfg *routerConfig) *mux.Router {
		r := mux.NewRouter()
		r.Use(requestid.NewRequestIDMiddlewareWithExtractor(customExtractor))

		cfg.buf = new(bytes.Buffer)
		cfg.logger = slog.New(slog.NewJSONHandler(cfg.buf, nil))

		// Now call NewLogging with only the logger and extraAttributes.
		r.Use(NewContextLogMiddleware(*cfg.logger, cfg.extraAttributes, nil))
		r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if l := GetLogger(r.Context()); l != nil {
				l.Info("test log message")
			}
			w.WriteHeader(http.StatusOK)
		})
		return r
	}

	BeforeAll(func() {
		for name, cfg := range testRoutersConfigurationMap {
			routersMap[name] = buildRouter(cfg)
		}
	})

	It("should inject a context logger and log a test message", func() {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set(requestid.RequestARMClientRequestIDHeader, "test-request-id")

		routersMap[defaultRouterName].ServeHTTP(w, req)

		cfg := testRoutersConfigurationMap[defaultRouterName]
		out := cfg.buf.String()
		fmt.Println("Output from default router:")
		fmt.Println(out)
		// Expect default fields (source always "CtxLog" now).
		Expect(out).To(ContainSubstring(`"source":"CtxLog"`))
		Expect(out).To(ContainSubstring(`"time":`))
		Expect(out).To(ContainSubstring(`"level":"INFO"`))
		// Verify the test log message.
		Expect(out).To(ContainSubstring("test log message"))
		Expect(w.Result().StatusCode).To(Equal(http.StatusOK))
	})

	It("should log operationID and correlationID from headers", func() {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set(requestid.RequestAcsOperationIDHeader, "test-operation-id")
		req.Header.Set(requestid.RequestCorrelationIDHeader, "test-correlation-id")

		routersMap[defaultRouterName].ServeHTTP(w, req)

		cfg := testRoutersConfigurationMap[defaultRouterName]
		out := cfg.buf.String()
		Expect(out).To(ContainSubstring(`"operationid":"test-operation-id"`))
		Expect(out).To(ContainSubstring(`"correlationid":"test-correlation-id"`))
		Expect(out).ToNot(ContainSubstring(`"armclientrequestid"`))
		Expect(w.Result().StatusCode).To(Equal(http.StatusOK))
	})

	It("should include custom attributes provided in the extraAttributes map", func() {
		// Test for routers configured with fixed extra attributes.
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set(requestid.RequestAcsOperationIDHeader, "test-op-id")
		req.Header.Set(requestid.RequestCorrelationIDHeader, "test-corr-id")

		routersMap[onlyAssignerRouterName].ServeHTTP(w, req)
		cfg := testRoutersConfigurationMap[onlyAssignerRouterName]
		out := cfg.buf.String()
		Expect(out).To(ContainSubstring(fmt.Sprintf(`"%s":"%s"`, customTestKey, customTestValue)))
		Expect(w.Result().StatusCode).To(Equal(http.StatusOK))

		// Also test the onlyInitializer configuration.
		w2 := httptest.NewRecorder()
		req2 := httptest.NewRequest("GET", "/", nil)
		routersMap[onlyInitializerRouterName].ServeHTTP(w2, req2)
		cfg2 := testRoutersConfigurationMap[onlyInitializerRouterName]
		out2 := cfg2.buf.String()
		Expect(out2).To(ContainSubstring(fmt.Sprintf(`"%s":"%s"`, customTestKey, customTestValue)))
		Expect(w2.Result().StatusCode).To(Equal(http.StatusOK))
	})

	It("should include default and extra attributes in logging for extra variables", func() {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set(requestid.RequestAcsOperationIDHeader, "test-operation-id")
		req.Header.Set(requestid.RequestCorrelationIDHeader, "test-correlation-id")
		req.Header.Set(requestid.RequestARMClientRequestIDHeader, "test-request-id")

		routersMap[extraLoggingVariablesName].ServeHTTP(w, req)
		cfg := testRoutersConfigurationMap[extraLoggingVariablesName]
		out := cfg.buf.String()
		Expect(out).To(ContainSubstring(`"source":"CtxLog"`))
		Expect(out).To(ContainSubstring(`"time":`))
		Expect(out).To(ContainSubstring(`"level":"INFO"`))
		Expect(out).To(ContainSubstring(fmt.Sprintf(`"request_id":"%s"`, "test-request-id")))
		Expect(out).To(ContainSubstring(`"method":"GET"`))
		// Verify extra attributes appear.
		Expect(out).To(ContainSubstring(fmt.Sprintf(`"%s":"%s"`, resourceGroupNameKey, "defaultRGnamevalue")))
		Expect(out).To(ContainSubstring(fmt.Sprintf(`"%s":"%s"`, subscriptionIDKey, "defaultSubIDvalue")))
		Expect(out).To(ContainSubstring(fmt.Sprintf(`"%s":"%s"`, errorDetailsKey, "defaultErrorDetailsvalue")))
		Expect(out).To(ContainSubstring(fmt.Sprintf(`"%s":%d`, resultTypeKey, 2)))
		Expect(w.Result().StatusCode).To(Equal(http.StatusOK))
	})
})
