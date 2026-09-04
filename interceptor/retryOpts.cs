using Grpc.Core;
using Grpc.Core.Interceptors;
using System;
using System.Collections.Generic;

namespace Azure.Aks.Middleware.Interceptors
{
    public class RetryInterceptor : Interceptor
    {
        private readonly int _maxRetries;
        private readonly TimeSpan _initialBackoff;
        private readonly double _backoffMultiplier;
        private readonly HashSet<StatusCode> _retryableStatusCodes;

        public RetryInterceptor(int maxRetries = 3, TimeSpan? initialBackoff = null, double backoffMultiplier = 1.5, IEnumerable<StatusCode> retryableStatusCodes = null)
        {
            _maxRetries = maxRetries;
            _initialBackoff = initialBackoff ?? TimeSpan.FromMilliseconds(100);
            _backoffMultiplier = backoffMultiplier;
            _retryableStatusCodes = new HashSet<StatusCode>(retryableStatusCodes ?? new[] { StatusCode.Unavailable, StatusCode.Aborted });
        }

        public override async Task<TResponse> UnaryServerHandler<TRequest, TResponse>(
            TRequest request,
            ServerCallContext context,
            UnaryServerMethod<TRequest, TResponse> continuation)
        {
            var retries = 0;
            var backoff = _initialBackoff;

            while (true)
            {
                try
                {
                    return await continuation(request, context);
                }
                catch (RpcException ex) when (_retryableStatusCodes.Contains(ex.StatusCode) && retries < _maxRetries)
                {
                    await Task.Delay(backoff);
                    backoff = TimeSpan.FromTicks((long)(backoff.Ticks * _backoffMultiplier));
                    retries++;
                }
            }
        }
    }
}
