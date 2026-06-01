package firebase

import (
	"context"

	"ego/platform/config"
	"ego/platform/logger"

	firebasev4 "firebase.google.com/go/v4"
	firebaseAuth "firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"
)

type Client struct {
	app  *firebasev4.App
	auth *firebaseAuth.Client
}

func NewClient(ctx context.Context, cfg *config.FirebaseConfig) (*Client, error) {
	opt := option.WithCredentialsFile(cfg.FirebaseCredentialsPath)
	app, err := firebasev4.NewApp(ctx, nil, opt)
	if err != nil {
		logger.Log.Error().Err(err).Msg("[FIREBASE] NewClient error")
		return nil, err
	}

	authClient, err := app.Auth(ctx)
	if err != nil {
		logger.Log.Error().Err(err).Msg("[FIREBASE] AuthClient error")
		return nil, err
	}

	return &Client{
		app:  app,
		auth: authClient,
	}, nil
}
