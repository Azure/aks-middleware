package customlogging

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCustomLogging(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Custom Logging Suite")
}
