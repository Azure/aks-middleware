using System;
using System.Net.Http;
using System.Threading.Tasks;
using Microsoft.Extensions.Logging;

namespace Azure.Aks.Middleware.Policy
{
    public class LoggingPolicy : DelegatingHandler
    {
        private readonly ILogger<LoggingPolicy> _logger;

        public LoggingPolicy(ILogger<LoggingPolicy> logger)
        {
            _logger = logger ?? throw new ArgumentNullException(nameof(logger));
        }

        protected override async Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, System.Threading.CancellationToken cancellationToken)
        {
            var startTime = DateTime.UtcNow;
            _logger.LogInformation($"Starting request to {request.RequestUri}");

            var response = await base.SendAsync(request, cancellationToken);

            var endTime = DateTime.UtcNow;
            _logger.LogInformation($"Completed request to {request.RequestUri} with status code {response.StatusCode} in {(endTime - startTime).TotalMilliseconds} ms");

            return response;
        }
    }
}
