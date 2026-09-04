using Grpc.Core;
using Grpc.Core.Interceptors;
using System.Threading.Tasks;

namespace Azure.Aks.Middleware.MdForward
{
    public class MdForwardInterceptor : Interceptor
    {
        public override async Task<TResponse> UnaryServerHandler<TRequest, TResponse>(
            TRequest request,
            ServerCallContext context,
            UnaryServerMethod<TRequest, TResponse> continuation)
        {
            var metadata = context.RequestHeaders;
            foreach (var entry in metadata)
            {
                context.ResponseTrailers.Add(entry);
            }

            return await continuation(request, context);
        }
    }
}
