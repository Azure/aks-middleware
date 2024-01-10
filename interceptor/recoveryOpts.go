package interceptor

import (
	"fmt"
	"net/url"
	"os"
	"runtime/debug"
	"strings"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func ParseStack(stack string) (string, string) {
	lines := strings.Split(stack, "panic.go") // split the stack into before panic and after panic
	trace := strings.Split(lines[1], "\n")    // split trace into lines
	// Example Input
	// panic({0xac4000, 0xd00610})
	// /usr/local/go1.19/src/runtime/panic.go:884 +0x212
	// go.goms.io/aks/rp/mygreeterv3/server/internal/server.(*Server).SayHello (0x0?, {0xd0a960?, 0xc0007fa270?}, 0xc00073d1d0)
	// /root/aks-rp/mygreeterv3/server/internal/server/api.go:34 +0x299
	var file = ""
	var linenum = ""
	for _, line := range trace {
		// file name and number of what caused panic
		if strings.Contains(line, ".go:") {
			parts := strings.Split(line, " ")
			fileAndLine := strings.Split(parts[0], ":")
			file = strings.TrimSpace(fileAndLine[0])
			linenum = fileAndLine[1]
			break
		}
	}

	return file, linenum
}

func GetRecoveryOpts() []recovery.Option {
	getFileAndLineNum := func(p any) (err error) {
		// get the file and line number where the panic occurred
		// panic is terminated by custom recovery function and program continues
		stack := debug.Stack()
		file, linenum := ParseStack(string(stack))

		// TODO: allow users to customize or we auto populate this repo path.
		base := "https://msazure.visualstudio.com/CloudNativeCompute/_git/aks-rp"
		path := file
		if strings.Contains(path, "aks-rp") {
			filepath := strings.SplitN(file, "aks-rp", 2)
			path = filepath[1]
		}

		version := "GB" + os.Getenv("AKS_BIN_VERSION_GITBRANCH")

		params := url.Values{}
		params.Add("path", path)

		params.Add("version", version)
		params.Add("line", linenum)

		query, err := url.QueryUnescape(params.Encode())
		if err != nil {
			fmt.Println(err)
			// skip query if url.QueryUnescape() fails
			query = ""
		}

		url := ""
		// url is not generated as file is not in aks-rp
		if path != file {
			url = base + "?" + query
		}

		// format the error message with the file and line number
		return status.Errorf(codes.Unknown, "panic_message: %v, file: %s, line: %s, url: %s", p, file, linenum, url)
	}
	opts := []recovery.Option{
		recovery.WithRecoveryHandler(getFileAndLineNum),
	}

	return opts

}
