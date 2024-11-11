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
				armClientRequestID string
				clientSessionID    string
			)
			if ok {
				if vals := md.Get(string(CorrelationIDKey)); len(vals) > 0 {
					correlationID = vals[0]
				}
				if vals := md.Get(string(ARMClientRequestIDKey)); len(vals) > 0 {
					armClientRequestID = vals[0]
				}
				if vals := md.Get(string(ClientSessionIDKey)); len(vals) > 0 {
					clientSessionID = vals[0]
				}
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(
				correlationID + "," +
					armClientRequestID + "," +
					clientSessionID,
			))
		})
		recorder = httptest.NewRecorder()
	})

	It("should extract all default headers", func() {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set(RequestCorrelationIDHeader, "test-correlation-id")
		req.Header.Set(RequestARMClientRequestIDHeader, "test-arm-client-request-id")
		req.Header.Set(RequestClientSessionIDHeader, "test-client-session-id")

		router.ServeHTTP(recorder, req)

		Expect(recorder.Code).To(Equal(http.StatusOK))
		Expect(recorder.Body.String()).To(Equal(
			"test-correlation-id," +
				"test-arm-client-request-id," +
				"test-client-session-id",
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
			RequestAcsOperationIDHeader = "x-ms-acs-operation-id"
		)

		customExtractor := func(r *http.Request) map[string]string {
			return map[string]string{
				string(CorrelationIDKey): r.Header.Get(RequestCorrelationIDHeader),
				"acsOperationID":         r.Header.Get(RequestAcsOperationIDHeader),
			}
		}

		// Set up a new router with the custom extractor
		customRouter := mux.NewRouter()
		customRouter.Use(NewRequestIDMiddlewareWithExtractor(customExtractor))
		customRouter.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			md, ok := metadata.FromIncomingContext(ctx)
			var (
				correlationID  string
				acsOperationID string
			)
			if ok {
				if vals := md.Get(string(CorrelationIDKey)); len(vals) > 0 {
					correlationID = vals[0]
				}
				if vals := md.Get("acsOperationID"); len(vals) > 0 {
					acsOperationID = vals[0]
				}
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(
				correlationID + "," +
					acsOperationID,
			))
		})
		customRecorder := httptest.NewRecorder()

		// Create a request with custom headers
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set(RequestCorrelationIDHeader, "custom-correlation-id")
		req.Header.Set(RequestAcsOperationIDHeader, "custom-acs-operation-id")

		customRouter.ServeHTTP(customRecorder, req)

		Expect(customRecorder.Code).To(Equal(http.StatusOK))
		Expect(customRecorder.Body.String()).To(Equal(
			"custom-correlation-id,custom-acs-operation-id",
		))
	})
})
