using Grpc.Core;
using Grpc.Core.Interceptors;
using Microsoft.Extensions.Logging;
using System;
using System.Collections.Generic;
using System.Linq;

namespace Azure.Aks.Middleware.CtxLogger
{
    public class CtxLoggerInterceptor : Interceptor
    {
        private readonly ILogger<CtxLoggerInterceptor> _logger;

        public CtxLoggerInterceptor(ILogger<CtxLoggerInterceptor> logger)
        {
            _logger = logger;
        }

        public override async Task<TResponse> UnaryServerHandler<TRequest, TResponse>(
            TRequest request,
            ServerCallContext context,
            UnaryServerMethod<TRequest, TResponse> continuation)
        {
            var method = context.Method;
            var requestHeaders = context.RequestHeaders.ToDictionary(header => header.Key, header => header.Value);
            var requestId = requestHeaders.ContainsKey("x-request-id") ? requestHeaders["x-request-id"] : "unknown";

            try
            {
                _logger.LogInformation($"Starting call. Method={method}, RequestId={requestId}");
                var response = await continuation(request, context);
                _logger.LogInformation($"Call finished successfully. Method={method}, RequestId={requestId}");
                return response;
            }
            catch (Exception ex)
            {
                _logger.LogError(ex, $"Call threw an exception. Method={method}, RequestId={requestId}");
                throw;
            }
        }
    }
}
