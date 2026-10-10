package push

import (
	"context"
	"errors"

	"firebase.google.com/go/v4/messaging"
)

type Sender interface {
	Send(ctx context.Context, tokens []string, title, body string, data map[string]string) ([]string, error)
}

type FCMSender struct{ client *messaging.Client }

func New(client *messaging.Client) *FCMSender { return &FCMSender{client: client} }

func (s *FCMSender) Send(ctx context.Context, tokens []string, title, body string, data map[string]string) ([]string, error) {
	if s == nil || s.client == nil || len(tokens) == 0 {
		return nil, nil
	}
	invalid := make([]string, 0)
	var errs []error
	for start := 0; start < len(tokens); start += 500 {
		end := min(start+500, len(tokens))
		batch := tokens[start:end]
		response, err := s.client.SendEachForMulticast(ctx, &messaging.MulticastMessage{
			Tokens: batch, Data: data,
			Notification: &messaging.Notification{Title: title, Body: body},
		})
		if err != nil {
			errs = append(errs, err)
			continue
		}
		for i, result := range response.Responses {
			if !result.Success {
				if messaging.IsUnregistered(result.Error) {
					invalid = append(invalid, batch[i])
				} else {
					errs = append(errs, result.Error)
				}
			}
		}
	}
	return invalid, errors.Join(errs...)
}
