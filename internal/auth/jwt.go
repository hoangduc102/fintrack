package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/hoangduc102/fintrack/internal/domain"
)

// JWTManager handles generating and validating HMAC-SHA256 JWT tokens.
type JWTManager struct {
	secretKey []byte
	duration  time.Duration
}

// NewJWTManager creates a new JWTManager.
func NewJWTManager(secretKey string, duration time.Duration) *JWTManager {
	if secretKey == "" {
		secretKey = "fintrack-default-dev-secret-key-32b"
	}
	return &JWTManager{
		secretKey: []byte(secretKey),
		duration:  duration,
	}
}

// Generate creates a 24-hour stateless HMAC-SHA256 token containing sub (user UUID), iat, and exp.
func (m *JWTManager) Generate(userID uuid.UUID) (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject:   userID.String(),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(m.duration)),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(m.secretKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

// Validate parses the token string, verifies its signature and claims, and returns the subject user UUID.
func (m *JWTManager) Validate(tokenString string) (uuid.UUID, error) {
	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return m.secretKey, nil
	})

	if err != nil || !token.Valid {
		return uuid.Nil, fmt.Errorf("%w: invalid or expired token", domain.ErrUnauthorized)
	}

	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok || claims.Subject == "" {
		return uuid.Nil, fmt.Errorf("%w: missing subject claim", domain.ErrUnauthorized)
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: invalid subject claim uuid: %v", domain.ErrUnauthorized, err)
	}

	return userID, nil
}
