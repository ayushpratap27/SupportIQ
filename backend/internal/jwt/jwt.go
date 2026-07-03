package jwt

import (
	"errors"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
)

// Claims is the JWT payload carried inside every token.
type Claims struct {
	UserID uint   `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	gojwt.RegisteredClaims
}

// TokenPair holds both the short-lived access token and the long-lived refresh token.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

// GenerateTokenPair creates a signed access token (15 min) and refresh token (7 days).
func GenerateTokenPair(userID uint, email, role, accessSecret, refreshSecret string) (*TokenPair, error) {
	access, err := newSignedToken(userID, email, role, 15*time.Minute, accessSecret)
	if err != nil {
		return nil, err
	}

	refresh, err := newSignedToken(userID, email, role, 7*24*time.Hour, refreshSecret)
	if err != nil {
		return nil, err
	}

	return &TokenPair{AccessToken: access, RefreshToken: refresh}, nil
}

// ValidateToken parses and verifies a token string, returning its claims on success.
func ValidateToken(tokenStr, secret string) (*Claims, error) {
	token, err := gojwt.ParseWithClaims(tokenStr, &Claims{}, func(t *gojwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*gojwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}
	return claims, nil
}

// newSignedToken is a private helper that builds and signs a single token.
func newSignedToken(userID uint, email, role string, ttl time.Duration, secret string) (string, error) {
	claims := &Claims{
		UserID: userID,
		Email:  email,
		Role:   role,
		RegisteredClaims: gojwt.RegisteredClaims{
			ExpiresAt: gojwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  gojwt.NewNumericDate(time.Now()),
		},
	}
	return gojwt.NewWithClaims(gojwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}
