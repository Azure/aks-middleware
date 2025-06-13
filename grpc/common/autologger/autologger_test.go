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
})
