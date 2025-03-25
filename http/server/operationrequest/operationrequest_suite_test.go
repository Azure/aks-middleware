package operationrequest

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestOperationRequest(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "OperationRequest Suite")
}
