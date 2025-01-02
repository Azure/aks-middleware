package mdforward_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestMdforward(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Mdforward Suite")
}
