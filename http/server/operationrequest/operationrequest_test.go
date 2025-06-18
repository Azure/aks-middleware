package operationrequest

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/Azure/aks-middleware/http/common"
	"github.com/gofrs/uuid"
	"github.com/gorilla/mux"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("OperationRequest Tests", func() {
	var (
		req      *http.Request
		router   *mux.Router
		validURL string
	)

	noOpCustomizer := OperationRequestCustomizerFunc(func(extras map[string]interface{}, headers http.Header, vars map[string]string) error {
		return nil
	})

	defaultOpts := OperationRequestOptions{
		Customizer: noOpCustomizer,
	}

	BeforeEach(func() {
		// setup a router for matching URL variables
		router = mux.NewRouter()
		routePattern := "/subscriptions/{subscriptionID}/resourceGroups/{resourceGroup}/providers/{resourceProvider}/{resourceType}/{resourceName}/default"
		validURL = "/subscriptions/sub3/resourceGroups/rg3/providers/Microsoft.Test/resourceType1/resourceName1/default?api-version=2021-12-01-Preview"
		router.HandleFunc(routePattern, func(w http.ResponseWriter, r *http.Request) {})
	})

	Describe("NewBaseOperationRequest", func() {
		It("should return an error when api-version is missing", func() {
			url := "/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Test/resourceType1/resourceName1/default"
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
			Expect(op.APIVersion).To(Equal("2021-12-01-preview"))
			Expect(op.SubscriptionID).To(Equal("sub3"))
			Expect(op.ResourceGroup).To(Equal("rg3"))
			Expect(op.CorrelationID).To(Equal("corr-test"))
			Expect(op.HttpMethod).To(Equal(http.MethodPost))
			Expect(op.TargetURI).To(ContainSubstring("api-version=2021-12-01-Preview"))
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

				customizer := OperationRequestCustomizerFunc(func(extras map[string]interface{}, headers http.Header, vars map[string]string) error {
					if customHeader := headers.Get("X-My-Custom-Header"); customHeader != "" {
						extras["MyCustomHeader"] = customHeader
					}
					// Attempt to modify extracted URI vars (this change should not persist in BaseOperationRequest)
					vars[common.SubscriptionIDKey] = "modified-subscription"
					return nil
				})

				opts := OperationRequestOptions{
					Customizer: customizer,
				}

				op, err := NewBaseOperationRequest(req, "region-custom", opts)
				Expect(err).NotTo(HaveOccurred())
				Expect(op).NotTo(BeNil())
				// Verify that custom field is added.
				Expect(op.Extras["MyCustomHeader"]).To(Equal("header-value"))

				// Verify extracted fields remain unchanged.
				Expect(op.APIVersion).To(Equal("2021-12-01-preview"))
				Expect(op.SubscriptionID).To(Equal("sub3"))
				Expect(op.ResourceGroup).To(Equal("rg3"))
				Expect(op.ResourceType).To(Equal("Microsoft.Test/resourceType1"))
				Expect(op.ResourceName).To(Equal("resourceName1"))
			})

			It("should propagate an error from the customizer", func() {
				req = httptest.NewRequest(http.MethodPost, validURL, nil)

				routeMatch := &mux.RouteMatch{}
				Expect(router.Match(req, routeMatch)).To(BeTrue())
				req = mux.SetURLVars(req, routeMatch.Vars)

				customErr := errors.New("custom error")
				customizer := OperationRequestCustomizerFunc(func(extras map[string]interface{}, headers http.Header, vars map[string]string) error {
					return customErr
				})

				opts := OperationRequestOptions{
					Customizer: customizer,
				}

				op, err := NewBaseOperationRequest(req, "region-custom", opts)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(customErr))
				Expect(op).To(BeNil())
			})
		})
	})

	Describe("Concurrent Access Tests", func() {
		It("should not have concurrent map writes when processing multiple requests", func() {
			// Create a customizer that writes to the extras map
			concurrentCustomizer := OperationRequestCustomizerFunc(func(extras map[string]interface{}, headers http.Header, vars map[string]string) error {
				// Simulate some processing and write to the extras map
				extras["test_key"] = "test_value"
				// Add more writes to increase chance of race condition if it exists
				for i := 0; i < 10; i++ {
					extras[fmt.Sprintf("key_%d", i)] = fmt.Sprintf("value_%d", i)
				}
				return nil
			})

			opts := OperationRequestOptions{
				Customizer: concurrentCustomizer,
			}

			// Create multiple requests
			numRequests := 100
			numGoroutines := 10
			requestsPerGoroutine := numRequests / numGoroutines

			// Channel to collect any panics
			panicChan := make(chan interface{}, numRequests)
			doneChan := make(chan bool, numGoroutines)

			// Run concurrent requests
			for g := 0; g < numGoroutines; g++ {
				go func(goroutineID int) {
					defer func() {
						if r := recover(); r != nil {
							panicChan <- r
						}
						doneChan <- true
					}()

					for i := 0; i < requestsPerGoroutine; i++ {
						// Create a new request for each iteration
						req := httptest.NewRequest("PUT", validURL, bytes.NewBufferString(`{"test": "data"}`))
						req.Header.Set(common.RequestCorrelationIDHeader, uuid.Must(uuid.NewV4()).String())
						req.Header.Set(common.RequestAcsOperationIDHeader, uuid.Must(uuid.NewV4()).String())
						req.Header.Set(common.RequestAcceptLanguageHeader, "en-US")

						routeMatch := &mux.RouteMatch{}
						Expect(router.Match(req, routeMatch)).To(BeTrue())
						req = mux.SetURLVars(req, routeMatch.Vars)

						// This should not panic with concurrent map writes
						op, err := NewBaseOperationRequest(req, "region-test", opts)
						Expect(err).ToNot(HaveOccurred())
						Expect(op).ToNot(BeNil())
						Expect(op.Extras).ToNot(BeNil())
						Expect(op.Extras["test_key"]).To(Equal("test_value"))
					}
				}(g)
			}

			// Wait for all goroutines to complete
			for i := 0; i < numGoroutines; i++ {
				<-doneChan
			}

			// Check if any panics occurred
			select {
			case panic := <-panicChan:
				Fail(fmt.Sprintf("Concurrent map write panic occurred: %v", panic))
			default:
				// No panics - test passed
			}
		})
	})
})
