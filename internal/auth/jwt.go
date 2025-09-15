package auth

import (
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// JWTClaims structure
type JWTClaims struct {
	Role string `json:"role"`
	jwt.StandardClaims
}

// AuthRequest structure
type AuthRequest struct {
	APIKey string `json:"api_key"`
}

// AuthResponse structure
type AuthResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// GenerateToken creates a JWT token with specified expiration time and role
func GenerateToken(expirationTime time.Time, role string) (string, error) {
	claims := &JWTClaims{
		Role: role,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationTime.Unix(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))

	return tokenString, err
}

// HandleAuthentication processes auth requests and returns JWT tokens
func HandleAuthentication(normalAPIKey string, adminAPIKey string, jwtSecret string, req AuthRequest) (string, string, time.Time, error) {
	// Determine role based on API key
	var role string
	if req.APIKey == adminAPIKey {
		role = "admin"
	} else if req.APIKey == normalAPIKey {
		role = "user"
	} else {
		return "", "", time.Time{}, fmt.Errorf("invalid API key")
	}

	// Generate JWT token valid for 1 hour (60 minutes)
	expirationTime := time.Now().Add(1 * time.Hour)
	tokenString, err := GenerateToken(expirationTime, role)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("error generating token: %w", err)
	}

	return tokenString, role, expirationTime, nil
}