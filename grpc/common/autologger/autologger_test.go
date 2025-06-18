package autologger_test

import (
	"bytes"
	"context"
	"encoding/json"

	log "log/slog"

	"github.com/Azure/aks-middleware/grpc/common/autologger"
	httpcommon "github.com/Azure/aks-middleware/http/common"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/metadata"
)

var _ = Describe("Autologger Tests", func() {
	var (
		buffer  *bytes.Buffer
		logger  *log.Logger
		handler *log.JSONHandler
		ctx     context.Context
	)

	BeforeEach(func() {
		buffer = &bytes.Buffer{}
		handler = log.NewJSONHandler(buffer, nil)
		logger = log.New(handler)
		ctx = context.Background()
	})

	It("should log headers when metadata is present in the context", func() {
		// Create incoming metadata with correlation and operation IDs
		md := metadata.Pairs(
			httpcommon.CorrelationIDKey, "test-correlation-id",
			httpcommon.OperationIDKey, "test-operation-id",
		)

		ctx = metadata.NewIncomingContext(ctx, md)

		interceptorLogger := autologger.InterceptorLogger(logger)
		interceptorLogger.Log(ctx, logging.LevelInfo, "test message", "key1", "value1")

		var logEntry map[string]interface{}
		err := json.Unmarshal(buffer.Bytes(), &logEntry)
		Expect(err).NotTo(HaveOccurred())

		// Verify the headers field contains the metadata
		Expect(logEntry).To(HaveKey("headers"))
		headers, ok := logEntry["headers"].(map[string]interface{})
		Expect(ok).To(BeTrue())
		Expect(headers).To(HaveKey(httpcommon.CorrelationIDKey))
		Expect(headers[httpcommon.CorrelationIDKey]).To(Equal("test-correlation-id"))
		Expect(headers).To(HaveKey(httpcommon.OperationIDKey))
		Expect(headers[httpcommon.OperationIDKey]).To(Equal("test-operation-id"))

		// Verify basic logging fields are present
		Expect(logEntry).To(HaveKeyWithValue("key1", "value1"))
		Expect(logEntry).To(HaveKeyWithValue("msg", "test message"))
		Expect(logEntry).To(HaveKeyWithValue("level", "INFO"))

	})

	It("should handle context without metadata correctly", func() {
		interceptorLogger := autologger.InterceptorLogger(logger)

		interceptorLogger.Log(ctx, logging.LevelInfo, "test message", "key1", "value1")

		var logEntry map[string]interface{}
		err := json.Unmarshal(buffer.Bytes(), &logEntry)
		Expect(err).NotTo(HaveOccurred())

		// Verify basic logging fields are present
		Expect(logEntry).To(HaveKeyWithValue("key1", "value1"))
		Expect(logEntry).To(HaveKeyWithValue("msg", "test message"))
		Expect(logEntry).To(HaveKeyWithValue("level", "INFO"))

		// Verify headers field is present but empty when no metadata exists
		Expect(logEntry).To(HaveKey("headers"))
		headers, ok := logEntry["headers"].(map[string]interface{})
		Expect(ok).To(BeTrue())
		Expect(headers).To(BeEmpty())
	})

	It("should use custom header extraction function when provided", func() {
		// Create incoming metadata
		md := metadata.Pairs(
			"x-tenant-id", "tenant-12345",
			"x-user-id", "user-67890",
			"x-session-id", "session-abcdef",
			"x-api-version", "v2.1",
			httpcommon.CorrelationIDKey, "test-correlation-id",
		)

		ctx = metadata.NewIncomingContext(ctx, md)

		// Define a custom headers function that extracts different headers
		customHeadersFunc := func(ctx context.Context) map[string]string {
			headers := make(map[string]string)
			if md, ok := metadata.FromIncomingContext(ctx); ok {
				// Extract only tenant-id, user-id, and api-version
				if vals := md.Get("x-tenant-id"); len(vals) > 0 {
					headers["tenant_id"] = vals[0]
				}
				if vals := md.Get("x-user-id"); len(vals) > 0 {
					headers["user_id"] = vals[0]
				}
				if vals := md.Get("x-api-version"); len(vals) > 0 {
					headers["api_version"] = vals[0]
				}
			}
			return headers
		}

		interceptorLogger := autologger.InterceptorLoggerWithHeadersFunc(logger, customHeadersFunc)
		interceptorLogger.Log(ctx, logging.LevelInfo, "test message", "key1", "value1")

		var logEntry map[string]interface{}
		err := json.Unmarshal(buffer.Bytes(), &logEntry)
		Expect(err).NotTo(HaveOccurred())

		// Verify the headers field contains only the custom extracted headers
		Expect(logEntry).To(HaveKey("headers"))
		headers, ok := logEntry["headers"].(map[string]interface{})
		Expect(ok).To(BeTrue())

		// Should contain the custom extracted headers
		Expect(headers).To(HaveKey("tenant_id"))
		Expect(headers["tenant_id"]).To(Equal("tenant-12345"))
		Expect(headers).To(HaveKey("user_id"))
		Expect(headers["user_id"]).To(Equal("user-67890"))
		Expect(headers).To(HaveKey("api_version"))
		Expect(headers["api_version"]).To(Equal("v2.1"))

		// Should NOT contain the headers that weren't extracted by custom function
		Expect(headers).NotTo(HaveKey("x-session-id"))
		Expect(headers).NotTo(HaveKey(httpcommon.CorrelationIDKey))

		// Verify basic logging fields are present
		Expect(logEntry).To(HaveKeyWithValue("key1", "value1"))
		Expect(logEntry).To(HaveKeyWithValue("msg", "test message"))
		Expect(logEntry).To(HaveKeyWithValue("level", "INFO"))
	})
})
