package jwt

import (
	"context"
	"ego/platform/config"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

type Manager interface {
	Generate(ctx context.Context, userID string) (string, string, error)
	Validate(ctx context.Context, token string) (string, error)
}

var (
	ErrInvalidToken = errors.New("token is invalid")
	ErrTokenExpired = errors.New("token is expired")
)

type manager struct {
	secretKey          []byte
	accessTokenExpiry  time.Duration
	refreshTokenExpiry time.Duration
}

func NewManager(cfg *config.JwtConfig) Manager {
	return &manager{
		secretKey:          []byte(cfg.JwtSecret),
		accessTokenExpiry:  cfg.AccessTokenExpiry,
		refreshTokenExpiry: cfg.RefreshTokenExpiry,
	}
}

type Claims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

func (m *manager) Generate(ctx context.Context, userID string) (string, string, error) {
	accessTokenClaims := &Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "ego",
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(m.accessTokenExpiry)),
			Subject:   userID,
		},
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessTokenClaims)
	accessTokenStr, err := accessToken.SignedString(m.secretKey)
	if err != nil {
		return "", "", err
	}

	refreshTokenClaims := &Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "ego",
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(m.refreshTokenExpiry)),
			Subject:   userID,
		},
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshTokenClaims)
	refreshTokenStr, err := refreshToken.SignedString(m.secretKey)
	if err != nil {
		return "", "", err
	}

	return accessTokenStr, refreshTokenStr, nil
}

func (m *manager) Validate(ctx context.Context, tokenString string) (string, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return m.secretKey, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return "", ErrTokenExpired
		}
		return "", err
	}

	if !token.Valid {
		return "", ErrInvalidToken
	}

	return claims.UserID, nil
}
