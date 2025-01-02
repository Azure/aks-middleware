package autologger_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAutologger(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Autologger Suite")
}
