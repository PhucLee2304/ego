package socket

import (
	"context"
	"encoding/json"
)

func JSONHandler[T any](fn func(ctx context.Context, client *Client, payload T) (any, error)) HandlerFunc {
	return func(ctx context.Context, client *Client, message Message) (any, error) {
		var payload T
		if len(message.Payload) > 0 {
			if err := json.Unmarshal(message.Payload, &payload); err != nil {
				return nil, NewHandlerError(ErrorCodeInvalidPayload, "invalid payload")
			}
		}

		return fn(ctx, client, payload)
	}
}
