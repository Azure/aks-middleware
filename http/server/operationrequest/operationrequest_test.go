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

var _ = Describe("OperationRequestHelper", func() {
    var (
        req    *http.Request
        router *mux.Router
    )

    BeforeEach(func() {
        // setup a router for matching URL variables
        router = mux.NewRouter()
        routePattern := "/subscriptions/{subscriptionId}/resourceGroups/{resourceGroup}/providers/{resourceProvider}/{resourceType}/{resourceName}/default"
        router.HandleFunc(routePattern, func(w http.ResponseWriter, r *http.Request) {})
    })

    Describe("NewBaseOperationRequest", func() {
        It("should return an error when api-version is missing", func() {
            url := "/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Test/providerType/resourceName/default"
            req = httptest.NewRequest(http.MethodGet, url, nil)
            req.Header.Set("x-ms-home-tenant-id", "tenant-test")

            routeMatch := &mux.RouteMatch{}
            Expect(router.Match(req, routeMatch)).To(BeTrue())
            req = mux.SetURLVars(req, routeMatch.Vars)

            op, err := NewBaseOperationRequest(req, "region-test", nil)
            Expect(err).To(HaveOccurred())
            Expect(err.Error()).To(ContainSubstring("no api-version in URI's parameters"))
            Expect(op).To(BeNil())
        })

        It("should correctly build a BaseOperationRequest", func() {
            url := "/subscriptions/sub2/resourceGroups/rg2/providers/Microsoft.Test/providerType/resourceName/default?api-version=2021-12-01"
            payload := "test payload"
            req = httptest.NewRequest(http.MethodPost, url, strings.NewReader(payload))
            req.Header.Set("x-ms-home-tenant-id", "tenant-test")
            req.Header.Set("x-ms-correlation-id", "corr-test")
            req.Header.Set("Accept-Language", "EN-GB")

            routeMatch := &mux.RouteMatch{}
            Expect(router.Match(req, routeMatch)).To(BeTrue())
            req = mux.SetURLVars(req, routeMatch.Vars)

            op, err := NewBaseOperationRequest(req, "region-test", nil)
            Expect(err).NotTo(HaveOccurred())
            Expect(op).NotTo(BeNil())
            Expect(op.APIVersion).To(Equal("2021-12-01"))
            Expect(op.TenantID).To(Equal("tenant-test"))
            Expect(op.SubscriptionID).To(Equal("sub2"))
            Expect(op.ResourceGroup).To(Equal("rg2"))
            Expect(op.CorrelationID).To(Equal("corr-test"))
            Expect(op.HttpMethod).To(Equal(http.MethodPost))
            Expect(op.TargetURI).To(ContainSubstring("api-version=2021-12-01"))
            Expect(strings.ToLower(op.AcceptedLanguage)).To(Equal("en-gb"))
            Expect(bytes.Equal(op.Body, []byte(payload))).To(BeTrue())
            Expect(op.OperationID).NotTo(BeEmpty())
        })

        It("should use the provided operation id if specified", func() {
            url := "/subscriptions/sub3/resourceGroups/rg3/providers/Microsoft.Test/providerType/resourceName/default?api-version=2021-12-01"
            req = httptest.NewRequest(http.MethodGet, url, nil)
            providedOpID := uuid.Must(uuid.NewV4()).String()
            req.Header.Set("x-ms-home-tenant-id", "tenant-test")
            req.Header.Set("x-ms-correlation-id", "corr-test")
            req.Header.Set(common.RequestAcsOperationIDHeader, providedOpID)
            req.Header.Set("Accept-Language", "EN-US")

            routeMatch := &mux.RouteMatch{}
            Expect(router.Match(req, routeMatch)).To(BeTrue())
            req = mux.SetURLVars(req, routeMatch.Vars)

            op, err := NewBaseOperationRequest(req, "region-test", nil)
            Expect(err).NotTo(HaveOccurred())
            Expect(op).NotTo(BeNil())
            Expect(op.OperationID).To(Equal(providedOpID))
        })

        Context("when using a customizer", func() {
            It("should apply customization to the BaseOperationRequest", func() {
                url := "/subscriptions/sub4/resourceGroups/rg4/providers/Microsoft.Test/providerType/resourceName/default?api-version=2021-12-01"
                req = httptest.NewRequest(http.MethodPost, url, strings.NewReader("payload"))
                req.Header.Set("x-ms-home-tenant-id", "tenant-custom")
                req.Header.Set("x-ms-correlation-id", "corr-custom")
                req.Header.Set("Accept-Language", "fr-FR")

                routeMatch := &mux.RouteMatch{}
                Expect(router.Match(req, routeMatch)).To(BeTrue())
                req = mux.SetURLVars(req, routeMatch.Vars)

                customizer := OperationRequestCustomizerFunc(func(op *BaseOperationRequest, headers http.Header, vars map[string]string) error {
                    op.Extras["custom"] = "value"
                    return nil
                })
                op, err := NewBaseOperationRequest(req, "region-custom", customizer)
                Expect(err).NotTo(HaveOccurred())
                Expect(op).NotTo(BeNil())
                Expect(op.Extras).To(HaveKeyWithValue("custom", "value"))
            })

            It("should propagate an error from the customizer", func() {
                url := "/subscriptions/sub5/resourceGroups/rg5/providers/Microsoft.Test/providerType/resourceName/default?api-version=2021-12-01"
                req = httptest.NewRequest(http.MethodPost, url, nil)
                req.Header.Set("x-ms-home-tenant-id", "tenant-customerror")

                routeMatch := &mux.RouteMatch{}
                Expect(router.Match(req, routeMatch)).To(BeTrue())
                req = mux.SetURLVars(req, routeMatch.Vars)

                customErr := errors.New("custom error")
                customizer := OperationRequestCustomizerFunc(func(op *BaseOperationRequest, headers http.Header, vars map[string]string) error {
                    return customErr
                })

                op, err := NewBaseOperationRequest(req, "region-custom", customizer)
                Expect(err).To(HaveOccurred())
                Expect(err).To(Equal(customErr))
                Expect(op).To(BeNil())
            })
        })
    })
})