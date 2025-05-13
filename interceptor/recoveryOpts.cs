using System;
using Grpc.Core;
using Grpc.Core.Interceptors;
using System.Diagnostics;
using System.Runtime.ExceptionServices;

namespace Azure.Aks.Middleware.Interceptors
{
    public class RecoveryInterceptor : Interceptor
    {
        private readonly Action<Exception> _exceptionHandler;

        public RecoveryInterceptor(Action<Exception> exceptionHandler = null)
        {
            _exceptionHandler = exceptionHandler ?? DefaultExceptionHandler;
        }

        private static void DefaultExceptionHandler(Exception exception)
        {
            // Log or handle the exception as needed
            Console.WriteLine($"Exception caught in GRPC interceptor: {exception.Message}");
        }

        public override async Task<TResponse> UnaryServerHandler<TRequest, TResponse>(
            TRequest request,
            ServerCallContext context,
            UnaryServerMethod<TRequest, TResponse> continuation)
        {
            try
            {
                return await continuation(request, context);
            }
            catch (Exception ex)
            {
                _exceptionHandler(ex);
                throw new RpcException(new Status(StatusCode.Internal, "An internal server error occurred."));
            }
        }
    }
}
