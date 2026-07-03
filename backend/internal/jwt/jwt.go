package jwt

import (
	"errors"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Claims is the JWT payload carried inside every token.
type Claims struct {
	UserID   uint      `json:"user_id"`
	TenantID uuid.UUID `json:"tenant_id"`
	Email    string    `json:"email"`
	Role     string    `json:"role"`
	gojwt.RegisteredClaims
}

// TokenPair holds both the short-lived access token and the long-lived refresh token.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

// GenerateTokenPair creates a signed access token (1 day) and refresh token (7 days).
func GenerateTokenPair(userID uint, tenantID uuid.UUID, email, role, accessSecret, refreshSecret string) (*TokenPair, error) {
	access, err := newSignedToken(userID, tenantID, email, role, 24*time.Hour, accessSecret)
	if err != nil {
		return nil, err
	}
	refresh, err := newSignedToken(userID, tenantID, email, role, 7*24*time.Hour, refreshSecret)
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

func newSignedToken(userID uint, tenantID uuid.UUID, email, role string, ttl time.Duration, secret string) (string, error) {
	claims := &Claims{
		UserID:   userID,
		TenantID: tenantID,
		Email:    email,
		Role:     role,
		RegisteredClaims: gojwt.RegisteredClaims{
			ExpiresAt: gojwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  gojwt.NewNumericDate(time.Now()),
		},
	}
	return gojwt.NewWithClaims(gojwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}
