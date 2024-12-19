package metadata

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Metadata", func() {
	Describe("extractMetadata", func() {
		var (
			headersToMetadata map[string]string
			req               *http.Request
			err               error
		)

		BeforeEach(func() {
			headersToMetadata = map[string]string{
				"X-Custom-Header": "custom-header",
			}
			req, err = http.NewRequest("GET", "http://example.com", nil)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should extract metadata from HTTP headers", func() {
			req.Header.Add("X-Custom-Header", "value")

			md := extractMetadata(headersToMetadata, req)
			Expect(md["custom-header"]).To(ContainElement("value"))
		})

		It("should ignore headers not in the headersToMetadata map", func() {
			req.Header.Add("X-Irrelevant-Header", "value")

			md := extractMetadata(headersToMetadata, req)
			Expect(md).NotTo(HaveKey("irrelevant-header"))
		})
	})

	Describe("matchOutgoingHeader", func() {
		var allowedHeaders map[string]string

		BeforeEach(func() {
			allowedHeaders = map[string]string{
				"custom-header": "X-Custom-Header",
			}
		})

		It("should match allowed headers", func() {
			header, ok := matchOutgoingHeader(allowedHeaders, "custom-header")
			Expect(ok).To(BeTrue())
			Expect(header).To(Equal("X-Custom-Header"))
		})

		It("should not match disallowed headers", func() {
			header, ok := matchOutgoingHeader(allowedHeaders, "other-header")
			Expect(ok).To(BeFalse())
			Expect(header).To(Equal(""))
		})
	})
})
