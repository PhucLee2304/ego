package rpc

import (
	"context"

	"ego/platform/logger"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const internalErrorMessage = "internal error"

// ErrorInterceptor is the last step of every gRPC server's error handling, so a
// handler only maps its own domain errors (PublicError / status.Error) and
// returns anything else unchanged:
//   - a gRPC status error is passed through as is;
//   - when the request context is cancelled or past its deadline, the matching
//     Canceled / DeadlineExceeded status is returned;
//   - any other error is logged with the method name and returned as Internal
//     with a generic message, so internal error text never leaves the service.
func ErrorInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		resp, err := handler(ctx, req)
		if err == nil {
			return resp, nil
		}
		if _, ok := status.FromError(err); ok {
			return nil, err
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, status.FromContextError(ctxErr).Err()
		}
		logger.Log.Error().Err(err).Str("method", info.FullMethod).Msg("[RPC] Request failed")
		return nil, status.Error(codes.Internal, internalErrorMessage)
	}
}
