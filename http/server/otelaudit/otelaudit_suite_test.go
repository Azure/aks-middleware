package otelaudit

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestOtelAudit(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Otel Audit Suite")
}
