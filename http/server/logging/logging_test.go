package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/Azure/aks-middleware/http/server/requestid"
	"github.com/gorilla/mux"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Httpmw", func() {
	var (
		router     *mux.Router
		buf        *bytes.Buffer
		slogLogger *slog.Logger
	)

	BeforeEach(func() {

		buf = new(bytes.Buffer)
		slogLogger = slog.New(slog.NewJSONHandler(buf, nil))

		router = mux.NewRouter()

		customExtractor := func(r *http.Request) map[string]string {
			return map[string]string{
				string(requestid.CorrelationIDKey): r.Header.Get(requestid.RequestCorrelationIDHeader),
				string(requestid.OperationIDKey):   r.Header.Get(requestid.RequestAcsOperationIDHeader),
			}
		}
		router.Use(requestid.NewRequestIDMiddlewareWithExtractor(customExtractor))
		router.Use(NewLogging(slogLogger))

		router.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "OK", http.StatusOK)
		})

		router.HandleFunc("/subscriptions/sub_id/resourceGroups/rg_name/providers/Microsoft.Storage/storageAccounts", func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "OK", http.StatusOK)
		})

		router.HandleFunc("/error", func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "test error", http.StatusBadRequest)
		})
	})

	Describe("LoggingMiddleware", func() {
		It("should log and return OK status", func() {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/subscriptions/sub_id/resourceGroups/rg_name/providers/Microsoft.Storage/storageAccounts?api-version=version&param1=value1&param2=value2", nil)

			router.ServeHTTP(w, req)

			Expect(buf.String()).To(ContainSubstring("finished call"))
			Expect(buf.String()).To(ContainSubstring(`"source":"ApiRequestLog"`))
			Expect(buf.String()).To(ContainSubstring(`"protocol":"HTTP"`))
			Expect(buf.String()).To(ContainSubstring(`"method":"GET storageaccounts - LIST"`))
			Expect(buf.String()).To(ContainSubstring(`"method_type":"unary"`))
			Expect(buf.String()).To(ContainSubstring(`"component":"server"`))
			Expect(buf.String()).To(ContainSubstring(`"time_ms":`))
			Expect(buf.String()).To(ContainSubstring(`"service":"`))
			Expect(buf.String()).To(ContainSubstring(`"url":"`))
			Expect(w.Result().StatusCode).To(Equal(http.StatusOK))
		})

		It("should log operationID and correlationID from headers", func() {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set(requestid.RequestAcsOperationIDHeader, "test-operation-id")
			req.Header.Set(requestid.RequestCorrelationIDHeader, "test-correlation-id")

			router.ServeHTTP(w, req)
			//runfunc()
			lines := strings.Split(buf.String(), "\n")
			var headersMap map[string]interface{}
			var err error
			for _, line := range lines {
				if strings.Contains(line, `"headers"`) {
					headersMap, err = unmarshalHeaders(line)
					Expect(err).ToNot(HaveOccurred(), "failed to unmarshal headers from log output")
					break
				}
			}

			Expect(headersMap["operationid"]).To(Equal("test-operation-id"))
			Expect(headersMap["correlationid"]).To(Equal("test-correlation-id"))
			Expect(buf.String()).ToNot(ContainSubstring(`"armclientrequestid"`))
			Expect(w.Result().StatusCode).To(Equal(http.StatusOK))
		})

		It("should capture error message for error responses", func() {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/error", nil)

			router.ServeHTTP(w, req)

			Expect(buf.String()).To(ContainSubstring("finished call"))
			Expect(buf.String()).To(ContainSubstring(`"code":400`))
			Expect(buf.String()).To(ContainSubstring("test error"))
			Expect(w.Result().StatusCode).To(Equal(http.StatusBadRequest))
		})
	})
})

type LogLine struct {
	Headers string `json:"headers"`
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
