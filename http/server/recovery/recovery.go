package recovery

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"runtime/debug"
	"strings"

	"github.com/Azure/aks-middleware/http/server/logging"
	"github.com/gorilla/mux"
)

const (
	// Panic logging info
	FilePathKey     = "FILE_PATH"
	PanicMessageKey = "MESSAGE"
)

type PanicHandlerFunc func(logger slog.Logger, w http.ResponseWriter, r *http.Request, err interface{})

func defaultPanicHandler(logger slog.Logger, w http.ResponseWriter, r *http.Request, err interface{}) {
	attributes := logging.BuildAttributes(r.Context(), r, "error", err)
	logger.ErrorContext(r.Context(), "Panic occurred", attributes...)
	http.Error(w, "Internal Server Error", http.StatusInternalServerError)
}

func NewPanicHandling(logger *slog.Logger, panicHandler PanicHandlerFunc) mux.MiddlewareFunc {
	if panicHandler == nil {
		panicHandler = defaultPanicHandler
	}
	return func(next http.Handler) http.Handler {
		return &panicHandlingMiddleware{
			next:         next,
			logger:       *logger,
			panicHandler: panicHandler,
		}
	}
}

type panicHandlingMiddleware struct {
	next         http.Handler
	logger       slog.Logger
	panicHandler PanicHandlerFunc
}

func (p *panicHandlingMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := recover(); err != nil {
			p.panicHandler(p.logger, w, r, err)
			r = SavePanicInfoToCtx(r, err, "")
		}
	}()
	p.next.ServeHTTP(w, r)
}

func parseStack(stack string) (string, string) {
	lines := strings.Split(stack, "panic.go") // split the stack into before panic and after panic
	trace := strings.Split(lines[1], "\n")    // split trace into lines
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

func SavePanicInfoToCtx(r *http.Request, err interface{}, stack string) *http.Request {
	// get the file and line number where the panic occurred
	// panic is terminated by custom recovery function and program continues
	if len(stack) == 0 {
		stack = string(debug.Stack())
	}

	file, linenum := parseStack(stack)

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

	ctx := r.Context()
	filePathLineNum := fmt.Sprintf("%s:%s", file, linenum)
	ctx = context.WithValue(ctx, FilePathKey, filePathLineNum)
	ctx = context.WithValue(ctx, PanicMessageKey, err)
	return r.WithContext(ctx)
}
