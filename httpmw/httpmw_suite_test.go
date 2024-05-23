package httpmw

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestHttpmw(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "HttpMW Suite")
}
