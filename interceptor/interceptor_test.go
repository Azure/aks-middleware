package interceptor_test

import (
	"github.com/Azure/aks-middleware/interceptor"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Recovery test", func() {
	Context("when a panic is triggered", func() {
		It("should parse the stack trace", func() {
			trace := `runtime/debug.Stack()
					/usr/local/go1.19/src/runtime/debug/stack.go:24 +0x65
				runtime/debug.PrintStack()
					/usr/local/go1.19/src/runtime/debug/stack.go:16 +0x19
				github.com/Azure/aks-middleware/interceptor.GetRecoveryOpts.func1({0xac4000, 0xd00610})
					/root/go/pkg/mod/github.com/Azure/aks-middleware@v0.1.16/interceptor/recoveryOpts.go:38 +0x3e
				github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery.WithRecoveryHandler.func1.1({0x40f45f?, 0xc0007fa270?}, {0xac4000?, 0xd00610?})
					/root/go/pkg/mod/github.com/grpc-ecosystem/go-grpc-middleware/v2@v2.0.0/interceptors/recovery/options.go:36 +0x2d
				github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery.recoverFrom({0xd0a960?, 0xc0007fa270?}, {0xac4000?, 0xd00610?}, 0xc00071b250?)
					/root/go/pkg/mod/github.com/grpc-ecosystem/go-grpc-middleware/v2@v2.0.0/interceptors/recovery/interceptors.go:54 +0x103
				github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery.UnaryServerInterceptor.func1.1()
					/root/go/pkg/mod/github.com/grpc-ecosystem/go-grpc-middleware/v2@v2.0.0/interceptors/recovery/interceptors.go:30 +0x68
				panic({0xac4000, 0xd00610})
					/usr/local/go1.19/src/runtime/panic.go:884 +0x212
				go.goms.io/aks/rp/mygreeterv3/server/internal/server.(*Server).SayHello(0x0?, {0xd0a960?, 0xc0007fa270?}, 0xc00073d1d0)
					/root/aks-rp/mygreeterv3/server/internal/server/api.go:34 +0x299
				go.goms.io/aks/rp/mygreeterv3/api/v1._MyGreeter_SayHello_Handler.func1({0xd0a960, 0xc0007fa270}, {0xb7b9c0?, 0xc00073d1d0})
					/root/aks-rp/mygreeterv3/api/v1/api_grpc.pb.go:92 +0x78
				github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery.UnaryServerInterceptor.func1({0xd0a960?, 0xc0007fa270?}, {0xb7b9c0?, 0xc00073d1d0?}, 0x0?, 0xc13a97a20e07f6e5?)
					/root/go/pkg/mod/github.com/grpc-ecosystem/go-grpc-middleware/v2@v2.0.0/interceptors/recovery/interceptors.go:34 +0xa7
				google.golang.org/grpc.getChainUnaryHandler.func1({0xd0a960, 0xc0007fa270}, {0xb7b9c0, 0xc00073d1d0})
					/root/go/pkg/mod/google.golang.org/grpc@v1.58.1/server.go:1195 +0xb9
				github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors.UnaryServerInterceptor.func1({0xd0a960, 0xc0007fa210}, {0xb7b9c0, 0xc00073d1d0}, 0xc00070e020?, 0xc0007ee940)
					/root/go/pkg/mod/github.com/grpc-ecosystem/go-grpc-middleware/v2@v2.0.0/interceptors/server.go:22 +0x2e3
				google.golang.org/grpc.getChainUnaryHandler.func1({0xd0a960, 0xc0007fa210}, {0xb7b9c0, 0xc00073d1d0})
					/root/go/pkg/mod/google.golang.org/grpc@v1.58.1/server.go:1195 +0xb9
				github.com/Azure/aks-middleware/ctxlogger.UnaryServerInterceptor.func1({0xd0a960, 0xc0007fa150}, {0xb7b9c0, 0xc00073d1d0}, 0xc00070e020?, 0xc0007ee900)
					/root/go/pkg/mod/github.com/Azure/aks-middleware@v0.1.16/ctxlogger/ctxlogger.go:57 +0xfe
				google.golang.org/grpc.getChainUnaryHandler.func1({0xd0a960, 0xc0007fa150}, {0xb7b9c0, 0xc00073d1d0})
					/root/go/pkg/mod/google.golang.org/grpc@v1.58.1/server.go:1195 +0xb9
				github.com/Azure/aks-middleware/requestid.UnaryServerInterceptor.func1({0xd0a960?, 0xc00069c330?}, {0xb7b9c0, 0xc00073d1d0}, 0xc00070e020?, 0xc0007ee8c0)
					/root/go/pkg/mod/github.com/Azure/aks-middleware@v0.1.16/requestid/requestid.go:30 +0x49
				google.golang.org/grpc.getChainUnaryHandler.func1({0xd0a960, 0xc00069c330}, {0xb7b9c0, 0xc00073d1d0})
					/root/go/pkg/mod/google.golang.org/grpc@v1.58.1/server.go:1195 +0xb9
				github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/protovalidate.UnaryServerInterceptor.func1({0xd0a960, 0xc00069c330}, {0xb7b9c0?, 0xc00073d1d0}, 0xc00070e020?, 0xc000696100)
					/root/go/pkg/mod/github.com/grpc-ecosystem/go-grpc-middleware/v2@v2.0.0/interceptors/protovalidate/protovalidate.go:40 +0x1ba
				google.golang.org/grpc.chainUnaryInterceptors.func1({0xd0a960, 0xc00069c330}, {0xb7b9c0, 0xc00073d1d0}, 0xc0008c5a20?, 0xb010a0?)
					/root/go/pkg/mod/google.golang.org/grpc@v1.58.1/server.go:1186 +0x8f
				go.goms.io/aks/rp/mygreeterv3/api/v1._MyGreeter_SayHello_Handler({0xb16780?, 0x12777f0}, {0xd0a960, 0xc00069c330}, 0xc0008223f0, 0xc00030bea0)
					/root/aks-rp/mygreeterv3/api/v1/api_grpc.pb.go:94 +0x138
				google.golang.org/grpc.(*Server).processUnaryRPC(0xc0003f4f00, {0xd0fdb8, 0xc00093cea0}, 0xc0004d0000, 0xc000962300, 0x122e880, 0x0)
					/root/go/pkg/mod/google.golang.org/grpc@v1.58.1/server.go:1376 +0xdf1
				google.golang.org/grpc.(*Server).handleStream(0xc0003f4f00, {0xd0fdb8, 0xc00093cea0}, 0xc0004d0000, 0x0)
					/root/go/pkg/mod/google.golang.org/grpc@v1.58.1/server.go:1753 +0xa2f
				google.golang.org/grpc.(*Server).serveStreams.func1.1()
					/root/go/pkg/mod/google.golang.org/grpc@v1.58.1/server.go:998 +0x98
				created by google.golang.org/grpc.(*Server).serveStreams.func1
					/root/go/pkg/mod/google.golang.org/grpc@v1.58.1/server.go:996 +0x18c`
			file, line := interceptor.ParseStack(trace)
			Expect(file).To(ContainSubstring("api.go"))
			Expect(line).To(Equal("34"))
		})
	})
})
