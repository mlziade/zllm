package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
)

// JWT claims structure
type JWTClaims struct {
	Role string `json:"role"`
	jwt.StandardClaims
}

// Authentication request structure
type AuthRequest struct {
	APIKey string `json:"api_key"`
}

// Authentication response structure
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

// JWTMiddleware authenticates requests using JWT tokens
func JWTMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get token from Authorization header
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(401).JSON(fiber.Map{"error": "Authorization header is required"})
		}

		// Check if the header has the "Bearer " prefix
		headerParts := strings.Split(authHeader, " ")
		if len(headerParts) != 2 || headerParts[0] != "Bearer" {
			return c.Status(401).JSON(fiber.Map{"error": "Invalid authorization header format"})
		}

		tokenString := headerParts[1]

		// Parse and validate the token
		token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(os.Getenv("JWT_SECRET")), nil
		})

		if err != nil {
			return c.Status(401).JSON(fiber.Map{"error": "Invalid or expired token"})
		}

		if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
			// Store user role in context for later use
			c.Locals("role", claims.Role)
			return c.Next()
		}

		return c.Status(401).JSON(fiber.Map{"error": "Invalid token"})
	}
}

// AdminMiddleware ensures the user has admin role
func AdminMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Check if the user has admin role
		role := c.Locals("role")
		if role != "admin" {
			return c.Status(403).JSON(fiber.Map{"error": "Admin access required"})
		}

		return c.Next()
	}
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
