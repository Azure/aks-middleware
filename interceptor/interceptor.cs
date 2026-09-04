using System;
using Grpc.Core;
using Grpc.Core.Interceptors;
using Microsoft.Extensions.Logging;
using System.Threading.Tasks;

namespace Azure.Aks.Middleware.Interceptors
{
    public class CustomInterceptor : Interceptor
    {
        private readonly ILogger<CustomInterceptor> _logger;

        public CustomInterceptor(ILogger<CustomInterceptor> logger)
        {
            _logger = logger;
        }

        public override async Task<TResponse> UnaryServerHandler<TRequest, TResponse>(
            TRequest request,
            ServerCallContext context,
            UnaryServerMethod<TRequest, TResponse> continuation)
        {
            _logger.LogInformation($"Starting call. Type: {typeof(TRequest).Name}");
            try
            {
                var response = await continuation(request, context);
                _logger.LogInformation($"Call completed successfully. Type: {typeof(TResponse).Name}");
                return response;
            }
            catch (Exception ex)
            {
                _logger.LogError(ex, $"An error occurred during the call. Type: {typeof(TRequest).Name}");
                throw;
            }
        }
    }
}
