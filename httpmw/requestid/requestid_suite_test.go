package requestid

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestRequestID(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "RequestID Suite")
}
