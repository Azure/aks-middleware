package operationid

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestOperationID(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "OperationID Suite")
}
