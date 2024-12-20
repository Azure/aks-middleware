package metadata

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Metadata", func() {
	Describe("extractMetadata", func() {
		var (
			headerToMetadata map[string]string
			req              *http.Request
			err              error
		)

		BeforeEach(func() {
			headerToMetadata = map[string]string{
				"X-Custom-Header": "custom-header",
			}
			req, err = http.NewRequest("GET", "http://example.com", nil)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should extract metadata from HTTP headers", func() {
			req.Header.Add("X-Custom-Header", "value")

			md := extractMetadata(headerToMetadata, req)
			Expect(md["custom-header"]).To(ContainElement("value"))
		})

		It("should ignore headers not in the headerToMetadata map", func() {
			req.Header.Add("X-Irrelevant-Header", "value")

			md := extractMetadata(headerToMetadata, req)
			Expect(md).NotTo(HaveKey("irrelevant-header"))
		})
	})

	Describe("matchOutgoingHeader", func() {
		var metadataToHeader map[string]string

		BeforeEach(func() {
			metadataToHeader = map[string]string{
				"custom-header": "X-Custom-Header",
			}
		})

		It("should match allowed headers", func() {
			header, ok := matchOutgoingHeader(metadataToHeader, "custom-header")
			Expect(ok).To(BeTrue())
			Expect(header).To(Equal("X-Custom-Header"))
		})

		It("should not match disallowed headers", func() {
			header, ok := matchOutgoingHeader(metadataToHeader, "other-header")
			Expect(ok).To(BeFalse())
			Expect(header).To(Equal(""))
		})
	})
})
