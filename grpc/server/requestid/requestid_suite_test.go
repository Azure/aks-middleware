package requestid_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestRequestid(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Requestid Suite")
}
