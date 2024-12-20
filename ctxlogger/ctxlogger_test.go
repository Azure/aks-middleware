package ctxlogger_test

import (
	"github.com/Azure/aks-middleware/ctxlogger"
	pb "github.com/Azure/aks-middleware/test/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Filtered logging test", func() {
	Context("when name/address is set as loggable=true", func() {
		It("should only print name, address, and address leaf nodes", func() {
			addr := &pb.Address{
				Street:  "123 Main St",
				City:    "Seattle",
				State:   "WA",
				Zipcode: int32(98101),
			}
			logs := ctxlogger.FilterLogs(&pb.HelloRequest{Name: "TestName", Age: 53, Email: "test@test.com", Address: addr})

			Expect(logs).To(BeEquivalentTo(map[string]interface{}{
				"address": map[string]interface{}{
					"city":    "Seattle",
					"zipcode": float64(98101),
				},
				"name": "TestName",
			})) // check if logs is equivalent to the expected map
		})

		It("should only print name and address", func() {
			addr := &pb.Address{}
			logs := ctxlogger.FilterLogs(&pb.HelloRequest{Name: "TestName", Age: 53, Email: "test@test.com", Address: addr})

			Expect(logs).To(BeEquivalentTo(map[string]interface{}{
				"address": map[string]interface{}{},
				"name":    "TestName",
			})) // check if logs is equivalent to the expected map
		})
	})
})
