package rpc

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestErrorInterceptor(t *testing.T) {
	expired, cancelExpired := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancelExpired()
	canceled, cancel := context.WithCancel(context.Background())
	cancel()

	tests := []struct {
		name    string
		ctx     context.Context
		err     error
		code    codes.Code
		message string
	}{
		{name: "public error passes through", ctx: context.Background(), err: PublicError(codes.NotFound, "[ERROR] User not found"), code: codes.NotFound, message: "[ERROR] User not found"},
		{name: "status error passes through", ctx: context.Background(), err: status.Error(codes.InvalidArgument, "bad id"), code: codes.InvalidArgument, message: "bad id"},
		{name: "plain error is hidden", ctx: context.Background(), err: errors.New("database password leaked"), code: codes.Internal, message: internalErrorMessage},
		{name: "deadline exceeded", ctx: expired, err: fmt.Errorf("query: %w", context.DeadlineExceeded), code: codes.DeadlineExceeded},
		{name: "canceled", ctx: canceled, err: errors.New("driver: bad connection"), code: codes.Canceled},
	}

	interceptor := ErrorInterceptor()
	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := interceptor(tt.ctx, nil, info, func(context.Context, any) (any, error) {
				return "partial", tt.err
			})
			if resp != nil {
				t.Fatalf("expected nil response on error, got %#v", resp)
			}
			if got := status.Code(err); got != tt.code {
				t.Fatalf("expected %s, got %s", tt.code, got)
			}
			if tt.message != "" && status.Convert(err).Message() != tt.message {
				t.Fatalf("expected message %q, got %q", tt.message, status.Convert(err).Message())
			}
		})
	}
}

func TestErrorInterceptorPassesSuccess(t *testing.T) {
	resp, err := ErrorInterceptor()(context.Background(), nil, &grpc.UnaryServerInfo{}, func(context.Context, any) (any, error) {
		return "ok", nil
	})
	if err != nil || resp != "ok" {
		t.Fatalf("expected ok response, got resp=%#v err=%v", resp, err)
	}
}
