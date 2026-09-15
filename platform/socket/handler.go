package socket

import (
	"context"
	"ego/platform/validatorx"
	"encoding/json"
	"fmt"

	"github.com/go-playground/validator/v10"
)

var validate = validatorx.New()

func JSONHandler[T any](fn func(ctx context.Context, client *Client, payload T) (any, error)) HandlerFunc {
	return func(ctx context.Context, client *Client, message Message) (any, error) {
		var payload T
		if len(message.Payload) > 0 {
			if err := json.Unmarshal(message.Payload, &payload); err != nil {
				return nil, NewHandlerError(ErrorCodeInvalidPayload, fmt.Sprintf("[SOCKET] Invalid payload format: %v", err))
			}
		}

		if err := validate.Struct(payload); err != nil {
			if _, ok := err.(*validator.InvalidValidationError); ok {
				return nil, NewHandlerError(ErrorCodeInternalError, fmt.Sprintf("[SOCKET] Invalid payload validator config: %v", err))
			}
			return nil, NewHandlerError(ErrorCodeInvalidPayload, fmt.Sprintf("[SOCKET] Invalid payload format: %v", err))
		}

		return fn(ctx, client, payload)
	}
}
