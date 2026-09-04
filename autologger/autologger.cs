using System;
using Grpc.Core;
using Grpc.Core.Interceptors;
using Microsoft.Extensions.Logging;

namespace Azure.Aks.Middleware.Autologger
{
    public class AutoLoggerInterceptor : Interceptor
    {
        private readonly ILogger _logger;

        public AutoLoggerInterceptor(ILogger<AutoLoggerInterceptor> logger)
        {
            _logger = logger;
        }

        public override async Task<TResponse> UnaryServerHandler<TRequest, TResponse>(
            TRequest request,
            ServerCallContext context,
            UnaryServerMethod<TRequest, TResponse> continuation)
        {
            try
            {
                LogCall(context.Method, LogLevel.Information, "Starting call.");
                var response = await continuation(request, context);
                LogCall(context.Method, LogLevel.Information, "Call finished successfully.");
                return response;
            }
            catch (Exception ex)
            {
                LogCall(context.Method, LogLevel.Error, $"Call threw an exception: {ex.Message}");
                throw;
            }
        }

        private void LogCall(string methodName, LogLevel logLevel, string message)
        {
            _logger.Log(logLevel, $"{methodName}: {message}");
        }
    }
}
