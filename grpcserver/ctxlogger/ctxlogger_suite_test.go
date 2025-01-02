package ctxlogger_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCtxlogger(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Ctxlogger Suite")
}
