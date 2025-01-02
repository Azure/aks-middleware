package restlogger_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestRestlogger(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Restlogger Suite")
}
