package handlers

import (
	"github.com/gofiber/fiber/v2"

	"zllm/internal/auth"
	"zllm/internal/config"
)

// HandleAuth processes authentication requests
func HandleAuth(cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Check if API keys are set
		if cfg.APIKey == "" || cfg.AdminAPIKey == "" {
			return c.Status(500).JSON(fiber.Map{"error": "API keys are not properly set in the .env file"})
		}

		// Check if JWT secret is set
		if cfg.JWTSecret == "" {
			return c.Status(500).JSON(fiber.Map{"error": "JWT_SECRET is not set in the .env file"})
		}

		// Parse the request body
		var req auth.AuthRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Error parsing request body"})
		}

		// Process the authentication request
		tokenString, role, expirationTime, err := auth.HandleAuthentication(cfg.APIKey, cfg.AdminAPIKey, cfg.JWTSecret, req)
		if err != nil {
			return c.Status(401).JSON(fiber.Map{"error": err.Error()})
		}

		// Return the token, role, and expiration time
		return c.JSON(fiber.Map{
			"token":      tokenString,
			"role":       role,
			"expires_at": expirationTime,
		})
	}
}