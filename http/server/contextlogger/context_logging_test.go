package contextlogger

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"

	"github.com/Azure/aks-middleware/http/server/requestid"
	"github.com/gorilla/mux"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type routerConfig struct {
	buf         *bytes.Buffer
	logger      *slog.Logger
	extractFunc func(ctx context.Context, r *http.Request) map[string]interface{}
}

const (
	subscriptionIDKey    = "subscriptionID"
	resourceGroupNameKey = "resourceGroupName"
	resultTypeKey        = "resultType"
	errorDetailsKey      = "errorDetails"

	defaultRouterName         = "default"
	extraLoggingVariablesName = "extra"
	customTestRouterName      = "custom"

	customTestKey   = "testKey"
	customTestValue = "testValue"
)

var _ = Describe("HttpmwWithCustomAttributeLogging", Ordered, func() {
	var (
		// custom extractor for request IDs used by the request id middleware.
		requestIDExtractor = func(r *http.Request) map[string]string {
			return map[string]string{
				string(requestid.CorrelationIDKey): r.Header.Get(requestid.RequestCorrelationIDHeader),
				string(requestid.OperationIDKey):   r.Header.Get(requestid.RequestAcsOperationIDHeader),
			}
		}

		// router configurations
		routerConfigs = map[string]*routerConfig{
			defaultRouterName: {
				extractFunc: nil,
			},
			extraLoggingVariablesName: {
				extractFunc: func(ctx context.Context, r *http.Request) map[string]interface{} {
					attrs := make(map[string]interface{})
					attrs[subscriptionIDKey] = "extractedSubIDvalue"
					attrs[resourceGroupNameKey] = "extractedRGnamevalue"
					attrs[resultTypeKey] = 3
					attrs[errorDetailsKey] = "extractedErrorDetailsvalue"
					return attrs
				},
			},
			customTestRouterName: {
				extractFunc: nil,
			},
		}

		routersMap = map[string]*mux.Router{}
	)

	buildRouter := func(cfg *routerConfig) *mux.Router {
		r := mux.NewRouter()
		r.Use(requestid.NewRequestIDMiddlewareWithExtractor(requestIDExtractor))

		cfg.buf = new(bytes.Buffer)
		// For customTestRouter, supply the logger with static attributes.
		if cfg == routerConfigs[customTestRouterName] {
			cfg.logger = slog.New(slog.NewJSONHandler(cfg.buf, nil)).With(customTestKey, customTestValue)
		} else {
			cfg.logger = slog.New(slog.NewJSONHandler(cfg.buf, nil))
		}

		r.Use(New(*cfg.logger, cfg.extractFunc))
		r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if l := GetLogger(r.Context()); l != nil {
				l.Info("test log message")
			}
			w.WriteHeader(http.StatusOK)
		})
		return r
	}

	BeforeAll(func() {
		for name, cfg := range routerConfigs {
			routersMap[name] = buildRouter(cfg)
		}
	})

	It("should inject a context logger and log a test message for the default router", func() {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set(requestid.RequestARMClientRequestIDHeader, "test-request-id")

		routersMap[defaultRouterName].ServeHTTP(w, req)

		out := routerConfigs[defaultRouterName].buf.String()
		Expect(out).To(ContainSubstring(`"source":"CtxLog"`))
		Expect(out).To(ContainSubstring(`"method":"GET"`))
		Expect(out).To(ContainSubstring("test log message"))
		Expect(w.Result().StatusCode).To(Equal(http.StatusOK))
	})

	It("should log fields from the extraction function when provided", func() {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set(requestid.RequestAcsOperationIDHeader, "test-operation-id")
		req.Header.Set(requestid.RequestCorrelationIDHeader, "test-correlation-id")
		req.Header.Set(requestid.RequestARMClientRequestIDHeader, "test-request-id")

		routersMap[extraLoggingVariablesName].ServeHTTP(w, req)

		out := routerConfigs[extraLoggingVariablesName].buf.String()
		// Check values from requestIDExtractor.
		Expect(out).To(ContainSubstring(`"operationid":"test-operation-id"`))
		Expect(out).To(ContainSubstring(`"correlationid":"test-correlation-id"`))
		// Verify extra extracted attributes appear.
		Expect(out).To(ContainSubstring(fmt.Sprintf(`"%s":"extractedRGnamevalue"`, resourceGroupNameKey)))
		Expect(out).To(ContainSubstring(fmt.Sprintf(`"%s":"extractedSubIDvalue"`, subscriptionIDKey)))
		Expect(out).To(ContainSubstring(fmt.Sprintf(`"%s":%d`, resultTypeKey, 3)))
		Expect(out).To(ContainSubstring(fmt.Sprintf(`"%s":"extractedErrorDetailsvalue"`, errorDetailsKey)))
		Expect(w.Result().StatusCode).To(Equal(http.StatusOK))
	})

	It("should include custom static attributes for the custom router", func() {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)

		routersMap[customTestRouterName].ServeHTTP(w, req)

		out := routerConfigs[customTestRouterName].buf.String()
		Expect(out).To(ContainSubstring(`"source":"CtxLog"`))
		Expect(out).To(ContainSubstring(`"method":"GET"`))
		Expect(out).To(ContainSubstring(fmt.Sprintf(`"%s":"%s"`, customTestKey, customTestValue)))
		Expect(w.Result().StatusCode).To(Equal(http.StatusOK))
	})

	It("Should be able to retrieve the logger already set in context with GetLogger()", func() {
		expectedLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
		ctx := WithLogger(context.Background(), expectedLogger)
		gotLogger := GetLogger(ctx)

		Expect(gotLogger).To(Equal(expectedLogger), "expected logger from context, got a different instance")
	})
})
