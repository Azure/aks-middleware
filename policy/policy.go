package policy

import (
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	armPolicy "github.com/Azure/azure-sdk-for-go/sdk/azcore/arm/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	azcorePolicy "github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/sirupsen/logrus"
)

type LoggingPolicy struct {
	logger logrus.FieldLogger
}

func NewLoggingPolicy(logger logrus.FieldLogger) *LoggingPolicy {
	return &LoggingPolicy{logger: logger}
}

func (p *LoggingPolicy) Do(req *azcorePolicy.Request) (*http.Response, error) {
	method := req.Raw().Method
	base := strings.Split(path.Base(req.Raw().URL.String()), "?")
	methodInfo := method + " " + base[0]
	parsedURL, parseErr := url.Parse(req.Raw().URL.String())
	if parseErr != nil {
		p.logger.Println("Error parsing request URL: ", parseErr)
		return nil, parseErr
	}
	startTime := time.Now()
	resp, err := req.Next()
	if err != nil {
		p.logger.Println("Error finishing request: ", err)
		return nil, err
	}

	// Time is in ms
	latency := time.Since(startTime).Milliseconds()

	p.logger.WithFields(logrus.Fields{
		"grpc.code":      resp.StatusCode,
		"grpc.component": "client",
		"grpc.time_ms":   latency,
		"grpc.method":    methodInfo,
		"grpc.service":   parsedURL.Host,
	}).Info("finished call")

	return resp, err
}

func (p *LoggingPolicy) Clone() azcorePolicy.Policy {
	return &LoggingPolicy{logger: p.logger}
}

func GetDefaultArmClientOptions() *armPolicy.ClientOptions {
	logOptions := new(policy.LogOptions)

	retryOptions := new(policy.RetryOptions)
	retryOptions.MaxRetries = 5

	clientOptions := new(policy.ClientOptions)
	clientOptions.Logging = *logOptions
	clientOptions.Retry = *retryOptions

	armClientOptions := new(armPolicy.ClientOptions)
	armClientOptions.ClientOptions = *clientOptions

	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	policyLogger := logger.WithFields(logrus.Fields{
		"source":           "ApiAutoLog",
		"grpc.method_type": "unary",
		"protocol":         "REST",
	})
	loggingPolicy := NewLoggingPolicy(policyLogger)

	armClientOptions.PerCallPolicies = append(armClientOptions.PerCallPolicies, loggingPolicy)

	return armClientOptions
}