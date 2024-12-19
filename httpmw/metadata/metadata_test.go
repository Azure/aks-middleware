package metadata

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Metadata", func() {
	Describe("extractMetadata", func() {
		It("should extract metadata from HTTP headers", func() {
			headersToMetadata := map[string]string{
				"X-Custom-Header": "custom-header",
			}
			req, err := http.NewRequest("GET", "http://example.com", nil)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Add("X-Custom-Header", "value")

			md := extractMetadata(headersToMetadata, req)
			Expect(md["custom-header"]).To(ContainElement("value"))
		})
	})

	Describe("matchOutgoingHeader", func() {
		It("should match allowed headers", func() {
			allowedHeaders := map[string]string{
				"custom-header": "X-Custom-Header",
			}

			header, ok := matchOutgoingHeader(allowedHeaders, "custom-header")
			Expect(ok).To(BeTrue())
			Expect(header).To(Equal("X-Custom-Header"))
		})

		It("should not match disallowed headers", func() {
			allowedHeaders := map[string]string{
				"custom-header": "X-Custom-Header",
			}

			header, ok := matchOutgoingHeader(allowedHeaders, "other-header")
			Expect(ok).To(BeFalse())
			Expect(header).To(Equal(""))
		})
	})
})
