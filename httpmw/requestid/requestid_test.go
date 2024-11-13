package requestid

import (
	"net/http"
	"net/http/httptest"

	"github.com/gorilla/mux"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/metadata"
)

var _ = Describe("RequestID Middleware", func() {
	var (
		router   *mux.Router
		recorder *httptest.ResponseRecorder
	)

	BeforeEach(func() {
		router = mux.NewRouter()
		router.Use(NewRequestIDMiddleware()) // Use default extractor
		router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			md, ok := metadata.FromIncomingContext(ctx)
			var (
				correlationID      string
				operationID        string
				armClientRequestID string
			)
			if ok {
				if vals := md.Get(string(CorrelationIDKey)); len(vals) > 0 {
					correlationID = vals[0]
				}
				if vals := md.Get(string(OperationIDKey)); len(vals) > 0 {
					operationID = vals[0]
				}
				if vals := md.Get(string(ARMClientRequestIDKey)); len(vals) > 0 {
					armClientRequestID = vals[0]
				}
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(
				correlationID + "," +
					operationID + "," +
					armClientRequestID,
			))
		})
		recorder = httptest.NewRecorder()
	})

	It("should extract all default headers", func() {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set(RequestCorrelationIDHeader, "test-correlation-id")
		req.Header.Set(RequestAcsOperationIDHeader, "test-operation-id")
		req.Header.Set(RequestARMClientRequestIDHeader, "test-arm-client-request-id")

		router.ServeHTTP(recorder, req)

		Expect(recorder.Code).To(Equal(http.StatusOK))
		Expect(recorder.Body.String()).To(Equal(
			"test-correlation-id," +
				"test-operation-id," +
				"test-arm-client-request-id",
		))
	})

	It("should handle missing headers gracefully", func() {
		req := httptest.NewRequest("GET", "/", nil)

		router.ServeHTTP(recorder, req)

		Expect(recorder.Code).To(Equal(http.StatusOK))
		Expect(recorder.Body.String()).To(Equal(",,"))
	})

	It("should extract custom headers with a custom extractor", func() {
		const (
			RequestCustomHeader = "x-ms-custom-id"
		)

		customExtractor := func(r *http.Request) map[string]string {
			return map[string]string{
				string(CorrelationIDKey): r.Header.Get(RequestCorrelationIDHeader),
				"customID":               r.Header.Get(RequestCustomHeader),
			}
		}

		// Set up a new router with the custom extractor
		customRouter := mux.NewRouter()
		customRouter.Use(NewRequestIDMiddlewareWithExtractor(customExtractor))
		customRouter.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			md, ok := metadata.FromIncomingContext(ctx)
			var (
				correlationID string
				customID      string
			)
			if ok {
				if vals := md.Get(string(CorrelationIDKey)); len(vals) > 0 {
					correlationID = vals[0]
				}
				if vals := md.Get("customID"); len(vals) > 0 {
					customID = vals[0]
				}
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(
				correlationID + "," +
					customID,
			))
		})
		customRecorder := httptest.NewRecorder()

		// Create a request with custom headers
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set(RequestCorrelationIDHeader, "custom-correlation-id")
		req.Header.Set(RequestCustomHeader, "test-custom-id")

		customRouter.ServeHTTP(customRecorder, req)

		Expect(customRecorder.Code).To(Equal(http.StatusOK))
		Expect(customRecorder.Body.String()).To(Equal(
			"custom-correlation-id,test-custom-id",
		))
	})
})
