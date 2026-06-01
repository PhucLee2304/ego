package firebase

import (
	"context"
	"ego/platform/logger"

	firebaseAuth "firebase.google.com/go/v4/auth"
	"github.com/aws/smithy-go/ptr"
)

type FirebaseUser struct {
	UID    string
	Email  string
	Name   *string
	Avatar *string
}

func (c *Client) VerifyIDToken(ctx context.Context, idToken string) (*FirebaseUser, error) {
	token, err := c.auth.VerifyIDToken(ctx, idToken)
	if err != nil {
		logger.Log.Error().Err(err).Msg("[FIREBASE] VerifyIDToken error")
		return nil, err
	}

	claims := token.Claims

	user := &FirebaseUser{
		UID: token.UID,
	}

	if email, ok := claims["email"].(string); ok {
		user.Email = email
	}

	if name, ok := claims["name"].(string); ok {
		user.Name = ptr.String(name)
	}

	if avatar, ok := claims["picture"].(string); ok {
		user.Avatar = ptr.String(avatar)
	}

	return user, nil
}

func (c *Client) GetUser(ctx context.Context, uid string) (*firebaseAuth.UserRecord, error) {
	user, err := c.auth.GetUser(ctx, uid)
	if err != nil {
		logger.Log.Error().Err(err).Msg("[FIREBASE] GetUser error")
		return nil, err
	}
	return user, nil
}
