package autologger_test

import (
	"bytes"
	"context"
	"encoding/json"

	log "log/slog"

	"github.com/Azure/aks-middleware/grpc/common/autologger"
	opreq "github.com/Azure/aks-middleware/http/server/operationrequest"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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

	It("should log correlation and operation IDs when OperationRequest is in the context", func() {
		// Create an OperationRequest with correlation and operation IDs
		opRequest := &opreq.BaseOperationRequest{
			CorrelationID: "test-correlation-id",
			OperationID:   "test-operation-id",
			Extras:        make(map[string]interface{}),
		}

		// Add the OperationRequest to the context
		ctx = opreq.OperationRequestWithContext(ctx, opRequest)

		// Setup the logger
		interceptorLogger := autologger.InterceptorLogger(logger)

		interceptorLogger.Log(ctx, logging.LevelInfo, "test message", "key1", "value1")

		var logEntry map[string]interface{}
		err := json.Unmarshal(buffer.Bytes(), &logEntry)
		Expect(err).NotTo(HaveOccurred())

		// Verify the headers field contains both IDs
		Expect(logEntry).To(HaveKey("headers"))
		headers, ok := logEntry["headers"].(map[string]interface{})
		Expect(ok).To(BeTrue())
		Expect(headers).To(HaveKey("correlation_id"))
		Expect(headers["correlation_id"]).To(Equal("test-correlation-id"))
		Expect(headers).To(HaveKey("operation_id"))
		Expect(headers["operation_id"]).To(Equal("test-operation-id"))

		// Verify that correlation_id and operation_id are NOT present as top-level attributes
		// They should only be in the headers field
		Expect(logEntry).NotTo(HaveKey("correlation_id"))
		Expect(logEntry).NotTo(HaveKey("operation_id"))

	})

	It("should handle empty correlation and operation IDs correctly", func() {
		// Create an OperationRequest with empty correlation and operation IDs
		opRequest := &opreq.BaseOperationRequest{
			CorrelationID: "",
			OperationID:   "",
			Extras:        make(map[string]interface{}),
		}

		ctx = opreq.OperationRequestWithContext(ctx, opRequest)

		interceptorLogger := autologger.InterceptorLogger(logger)

		interceptorLogger.Log(ctx, logging.LevelInfo, "test message", "key1", "value1")

		var logEntry map[string]interface{}
		err := json.Unmarshal(buffer.Bytes(), &logEntry)
		Expect(err).NotTo(HaveOccurred())

		// Verify the correlation and operation IDs are not in the log
		Expect(logEntry).NotTo(HaveKey("correlation_id"))
		Expect(logEntry).NotTo(HaveKey("operation_id"))
		Expect(logEntry).NotTo(HaveKey("headers"))
	})
})
