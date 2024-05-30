package logging_test

import (
	"github.com/Azure/aks-middleware/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("GetMethodInfo", func() {
	Context("when method is GET and URL has nested resource type", func() {
		It("returns the correct method info for a READ operation", func() {
			method := "GET"
			url := "https://management.azure.com/subscriptions/sub_id/resourceGroups/rg_name/providers/Microsoft.Storage/storageAccounts/account_name?api-version=version"
			expected := "GET storageAccounts - READ"
			Expect(logging.GetMethodInfo(method, url)).To(Equal(expected))
		})
	})

	Context("when method is GET and URL has top-level resource type", func() {
		It("returns the correct method info for a LIST operation", func() {
			method := "GET"
			url := "https://management.azure.com/subscriptions/sub_id/resourceGroups?api-version=version"
			expected := "GET resourceGroups - LIST"
			Expect(logging.GetMethodInfo(method, url)).To(Equal(expected))
		})
	})

	Context("when method is not GET", func() {
		It("returns the correct method info without operation type", func() {
			method := "POST"
			url := "https://management.azure.com/subscriptions/sub_id/resourceGroups/rg_name/providers/Microsoft.Storage/storageAccounts?api-version=version"
			expected := "POST storageAccounts"
			Expect(logging.GetMethodInfo(method, url)).To(Equal(expected))
		})
	})
})
