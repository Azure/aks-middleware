package operationrequest

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/Azure/aks-middleware/http/common"
	"github.com/gofrs/uuid"
	"github.com/gorilla/mux"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type MyExtras struct {
	MyCustomHeader string
}

var _ = Describe("OperationRequest using MyExtras", func() {
	var (
		req      *http.Request
		router   *mux.Router
		validURL string
	)

	// no-op customizer for MyExtras
	noOpCustomizer := OperationRequestCustomizerFunc[MyExtras](func(extras *MyExtras, headers http.Header, vars map[string]string) error {
		return nil
	})

	defaultOpts := OperationRequestOptions[MyExtras]{
		Extras:     MyExtras{},
		Customizer: noOpCustomizer,
	}

	BeforeEach(func() {
		// setup a router for matching URL variables
		router = mux.NewRouter()
		routePattern := "/subscriptions/{subscriptionId}/resourceGroups/{resourceGroup}/providers/{resourceProvider}/{resourceType}/{resourceName}/default"
		validURL = "/subscriptions/sub3/resourceGroups/rg3/providers/Microsoft.Test/providerType1/resourceName1/default?api-version=2021-12-01"
		router.HandleFunc(routePattern, func(w http.ResponseWriter, r *http.Request) {})
	})

	Describe("NewBaseOperationRequest", func() {
		It("should return an error when api-version is missing", func() {
			url := "/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Test/providerType1/resourceName1/default"
			req = httptest.NewRequest(http.MethodGet, url, nil)

			routeMatch := &mux.RouteMatch{}
			Expect(router.Match(req, routeMatch)).To(BeTrue())
			req = mux.SetURLVars(req, routeMatch.Vars)

			op, err := NewBaseOperationRequest(req, "region-test", defaultOpts)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no api-version in URI's parameters"))
			Expect(op).To(BeNil())
		})

		It("should correctly build a BaseOperationRequest", func() {
			payload := "test payload"
			req = httptest.NewRequest(http.MethodPost, validURL, strings.NewReader(payload))
			req.Header.Set(common.RequestCorrelationIDHeader, "corr-test")
			req.Header.Set(common.RequestAcceptLanguageHeader, "EN-GB")

			routeMatch := &mux.RouteMatch{}
			Expect(router.Match(req, routeMatch)).To(BeTrue())
			req = mux.SetURLVars(req, routeMatch.Vars)

			op, err := NewBaseOperationRequest(req, "region-test", defaultOpts)
			Expect(err).NotTo(HaveOccurred())
			Expect(op).NotTo(BeNil())
			Expect(op.APIVersion).To(Equal("2021-12-01"))
			Expect(op.SubscriptionID).To(Equal("sub3"))
			Expect(op.ResourceGroup).To(Equal("rg3"))
			Expect(op.CorrelationID).To(Equal("corr-test"))
			Expect(op.HttpMethod).To(Equal(http.MethodPost))
			Expect(op.TargetURI).To(ContainSubstring("api-version=2021-12-01"))
			Expect(strings.ToLower(op.AcceptedLanguage)).To(Equal("en-gb"))
			Expect(bytes.Equal(op.Body, []byte(payload))).To(BeTrue())
			Expect(op.OperationID).NotTo(BeEmpty())
		})

		It("should use the provided operation id if specified", func() {
			req = httptest.NewRequest(http.MethodGet, validURL, nil)
			providedOpID := uuid.Must(uuid.NewV4()).String()
			req.Header.Set(common.RequestCorrelationIDHeader, "corr-test")
			req.Header.Set(common.RequestAcsOperationIDHeader, providedOpID)
			req.Header.Set(common.RequestAcceptLanguageHeader, "EN-US")

			routeMatch := &mux.RouteMatch{}
			Expect(router.Match(req, routeMatch)).To(BeTrue())
			req = mux.SetURLVars(req, routeMatch.Vars)

			op, err := NewBaseOperationRequest(req, "region-test", defaultOpts)
			Expect(err).NotTo(HaveOccurred())
			Expect(op).NotTo(BeNil())
			Expect(op.OperationID).To(Equal(providedOpID))
		})

		Context("when using a customizer", func() {
			It("should apply customization to grab extra info from the request header and keep other vars intact", func() {
				req = httptest.NewRequest(http.MethodPost, validURL, strings.NewReader("payload"))
				req.Header.Set(common.RequestCorrelationIDHeader, "corr-custom")
				req.Header.Set(common.RequestAcceptLanguageHeader, "fr-FR")
				// Custom information to be extracted
				req.Header.Set("X-My-Custom-Header", "header-value")

				routeMatch := &mux.RouteMatch{}
				Expect(router.Match(req, routeMatch)).To(BeTrue())
				req = mux.SetURLVars(req, routeMatch.Vars)

				customizer := OperationRequestCustomizerFunc[MyExtras](func(extras *MyExtras, headers http.Header, vars map[string]string) error {
					if customHeader := headers.Get("X-My-Custom-Header"); customHeader != "" {
						extras.MyCustomHeader = customHeader
					}
					// Attempt to modify extracted URI vars (this change should not persist in BaseOperationRequest)
					vars[common.SubscriptionIDKey] = "modified-subscription"
					return nil
				})

				opts := OperationRequestOptions[MyExtras]{
					Extras:     MyExtras{},
					Customizer: customizer,
				}

				op, err := NewBaseOperationRequest(req, "region-custom", opts)
				Expect(err).NotTo(HaveOccurred())
				Expect(op).NotTo(BeNil())
				// Verify that custom field is added.
				Expect(op.Extras.MyCustomHeader).To(Equal("header-value"))

				// Verify extracted fields remain unchanged.
				Expect(op.APIVersion).To(Equal("2021-12-01"))
				Expect(op.SubscriptionID).To(Equal("sub3"))
				Expect(op.ResourceGroup).To(Equal("rg3"))
				Expect(op.ResourceType).To(Equal("Microsoft.Test/providerType1"))
				Expect(op.ResourceName).To(Equal("resourceName1"))
			})

			It("should propagate an error from the customizer", func() {
				req = httptest.NewRequest(http.MethodPost, validURL, nil)

				routeMatch := &mux.RouteMatch{}
				Expect(router.Match(req, routeMatch)).To(BeTrue())
				req = mux.SetURLVars(req, routeMatch.Vars)

				customErr := errors.New("custom error")
				customizer := OperationRequestCustomizerFunc[MyExtras](func(extras *MyExtras, headers http.Header, vars map[string]string) error {
					return customErr
				})

				opts := OperationRequestOptions[MyExtras]{
					Extras:     MyExtras{},
					Customizer: customizer,
				}

				op, err := NewBaseOperationRequest(req, "region-custom", opts)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(customErr))
				Expect(op).To(BeNil())
			})
		})
	})
})
