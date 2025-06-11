package contextlogger

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"

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

// should not be able to marshal this type to a string for logging
type InvalidType struct {
	Fn func()
}

const (
	subscriptionIDKey    = "subscriptionID"
	resourceGroupNameKey = "resourceGroupName"
	resultTypeKey        = "resultType"
	errorDetailsKey      = "errorDetails"

	defaultRouterName         = "default"
	extraLoggingVariablesName = "extra"
	extraLoggingCannotMarshal = "extraCannotMarshal"
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
			extraLoggingCannotMarshal: {
				extractFunc: func(ctx context.Context, r *http.Request) map[string]interface{} {
					attrs := make(map[string]interface{})
					attrs[subscriptionIDKey] = "extractedSubIDvalue"
					attrs[resourceGroupNameKey] = InvalidType{Fn: func() {
						panic("cannot marshal this value")
					}}
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
		logInfo, err := unmarshalLog(out)
		Expect(err).NotTo(HaveOccurred(), "failed to parse log string")

		lines := strings.Split(out, "\n")
		var headersMap map[string]interface{}
		for _, line := range lines {
			if strings.Contains(line, `"headers"`) {
				headersMap, err = unmarshalHeaders(line)
				Expect(err).ToNot(HaveOccurred(), "failed to unmarshal headers from log output")
				break
			}
		}
		Expect(headersMap["operationid"]).To(Equal("test-operation-id"))
		Expect(headersMap["correlationid"]).To(Equal("test-correlation-id"))
		// Verify extra extracted attributes appear.
		Expect(logInfo[resourceGroupNameKey]).To(Equal("extractedRGnamevalue"))
		Expect(logInfo[subscriptionIDKey]).To(Equal("extractedSubIDvalue"))
		Expect(logInfo[resultTypeKey]).To(Equal(float64(3)))
		Expect(logInfo[errorDetailsKey]).To(Equal("extractedErrorDetailsvalue"))
		Expect(w.Result().StatusCode).To(Equal(http.StatusOK))
	})

	It("should continue logging even if an attribute cannot be marshaled", func() {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set(requestid.RequestAcsOperationIDHeader, "test-operation-id")
		req.Header.Set(requestid.RequestCorrelationIDHeader, "test-correlation-id")
		req.Header.Set(requestid.RequestARMClientRequestIDHeader, "test-request-id")

		routersMap[extraLoggingCannotMarshal].ServeHTTP(w, req)

		out := routerConfigs[extraLoggingCannotMarshal].buf.String()
		_, err := unmarshalLog(out)
		Expect(err).To(HaveOccurred(), "failed to parse log string")
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

func unmarshalLog(out string) (map[string]interface{}, error) {
	var outer map[string]interface{}
	if err := json.Unmarshal([]byte(out), &outer); err != nil {
		return nil, fmt.Errorf("failed to unmarshal log output: %w", err)
	}
	logStr, ok := outer["log"].(string)
	if !ok {
		return nil, fmt.Errorf("log key not found or not a string in log output")
	}
	var inner map[string]interface{}
	err := json.Unmarshal([]byte(logStr), &inner)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal log string: %w", err)
	}
	return inner, nil
}

func unmarshalHeaders(log string) (map[string]interface{}, error) {
	fmt.Println("headers to unmarshal: ", log)
	var outer map[string]interface{}
	if err := json.Unmarshal([]byte(log), &outer); err != nil {
		fmt.Println("failed here==")
		return nil, fmt.Errorf("failed to unmarshal headers log output: %w", err)
	}
	headersStr, ok := outer["headers"]
	if !ok {
		return nil, fmt.Errorf("headers key not found or not a string in log output")
	}
	var inner map[string]interface{}
	err := json.Unmarshal([]byte(headersStr.(string)), &inner)
	if err != nil {
		fmt.Println("failed here 2==")

		return nil, fmt.Errorf("failed to unmarshal headers log string: %w", err)
	}
	return inner, nil
}
