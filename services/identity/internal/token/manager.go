package token

import (
	"context"
	"ego/services/identity/config"
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

type Manager interface {
	Generate(ctx context.Context, userID string) (string, string, error)
	// Validate accepts access tokens only.
	Validate(ctx context.Context, token string) (string, error)
	// ValidateRefresh accepts refresh tokens only.
	ValidateRefresh(ctx context.Context, token string) (string, error)
}

const (
	accessTokenType  = "access"
	refreshTokenType = "refresh"
)

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
	UserID    string `json:"user_id"`
	TokenType string `json:"typ"`
	jwt.RegisteredClaims
}

func (m *manager) Generate(ctx context.Context, userID string) (string, string, error) {
	accessTokenClaims := &Claims{
		UserID:    userID,
		TokenType: accessTokenType,
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
		UserID:    userID,
		TokenType: refreshTokenType,
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
	return m.validate(tokenString, accessTokenType)
}

func (m *manager) ValidateRefresh(ctx context.Context, tokenString string) (string, error) {
	return m.validate(tokenString, refreshTokenType)
}

func (m *manager) validate(tokenString, tokenType string) (string, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, ErrInvalidToken
		}
		return m.secretKey, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return "", ErrTokenExpired
		}
		return "", ErrInvalidToken
	}

	// Tokens issued before the "typ" claim existed carry no type; accept them until
	// they have all expired so existing sessions are not logged out.
	if !token.Valid || (claims.TokenType != "" && claims.TokenType != tokenType) {
		return "", ErrInvalidToken
	}

	userID := strings.TrimSpace(claims.UserID)
	if userID == "" {
		return "", ErrInvalidToken
	}

	return userID, nil
}
