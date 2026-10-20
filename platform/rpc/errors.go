package rpc

import (
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// PublicError builds a gRPC status whose message is explicitly marked as safe to
// show to end users. httpx.FromError forwards only this marked message; a plain
// status message is treated as internal and never reaches the client.
func PublicError(code codes.Code, message string) error {
	st := status.New(code, message)
	withDetails, err := st.WithDetails(&errdetails.LocalizedMessage{Locale: "en-US", Message: message})
	if err != nil {
		return st.Err()
	}
	return withDetails.Err()
}
