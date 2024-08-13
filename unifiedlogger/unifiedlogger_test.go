package logger

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("NewLoggerWrapper", func() {
	It("should return a LoggerWrapper with LogrusLogger when loggerType is 'logrus'", func() {
		ctx := context.Background()
		loggerType := "logrus"

		loggerWrapper := NewLoggerWrapper(loggerType, ctx)

		Expect(loggerWrapper.logger).To(BeAssignableToTypeOf(&LogrusLogger{}))
	})

	It("should return a LoggerWrapper with SlogLogger when loggerType is 'slog'", func() {
		ctx := context.Background()
		loggerType := "slog"

		loggerWrapper := NewLoggerWrapper(loggerType, ctx)

		Expect(loggerWrapper.logger).To(BeAssignableToTypeOf(&SlogLogger{}))
	})

	It("should return a LoggerWrapper with LogrusLogger when loggerType is not 'logrus' or 'slog'", func() {
		ctx := context.Background()
		loggerType := "unknown"

		loggerWrapper := NewLoggerWrapper(loggerType, ctx)

		Expect(loggerWrapper.logger).To(BeAssignableToTypeOf(&LogrusLogger{}))
	})
})

func TestLogger(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Logger Suite")
}
