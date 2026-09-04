using System;
using Grpc.Core;
using System.Threading.Tasks;

namespace Azure.Aks.Middleware
{
    public static class RequestId
    {
        private const string RequestIdMetadataKey = "x-request-id";
        private const string RequestIdLogKey = "request-id";

        public static ServerCallContext GenerateRequestId(ServerCallContext context)
        {
            var requestId = Guid.NewGuid().ToString();
            context.RequestHeaders.Add(RequestIdMetadataKey, requestId);
            return context;
        }

        public static string GetRequestId(Metadata headers)
        {
            var requestIdEntry = headers.FirstOrDefault(entry => entry.Key.Equals(RequestIdMetadataKey, StringComparison.OrdinalIgnoreCase));
            return requestIdEntry?.Value;
        }
    }
}
