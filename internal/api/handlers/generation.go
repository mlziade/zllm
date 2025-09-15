package handlers

import (
	"bufio"
	"strings"

	"github.com/gofiber/fiber/v2"

	"zllm/internal/ollama"
)

// HandleGeneration processes text generation requests
func HandleGeneration(client *ollama.Client) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Parse the request body
		var req ollama.GenerationRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).SendString("Error parsing request body")
		}

		// Validate required fields
		if req.Prompt == "" {
			return c.Status(400).SendString("Prompt is required")
		}
		if req.Model == "" {
			return c.Status(404).SendString("Model is required")
		}

		// Generate the response
		response, err := client.GenerateResponse(req)
		if err != nil {
			if strings.Contains(err.Error(), "model not found") {
				return c.Status(404).JSON(fiber.Map{"error": "Model not found"})
			}
			if strings.Contains(err.Error(), "model requires more system memory") {
				return c.Status(507).JSON(fiber.Map{"error": "Model requires more system memory"})
			}
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(response)
	}
}

// HandleGenerationStream processes streaming text generation requests
func HandleGenerationStream(client *ollama.Client) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Parse the request body
		var req ollama.GenerationRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).SendString("Error parsing request body")
		}

		// Validate required fields
		if req.Prompt == "" {
			return c.Status(400).SendString("Prompt is required")
		}
		if req.Model == "" {
			return c.Status(404).SendString("Model is required")
		}

		// Set headers for streaming
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")

		// Create a buffered writer for streaming
		writer := bufio.NewWriter(c.Response().BodyWriter())

		// Stream the response
		err := client.StreamGenerationResponse(req, writer)
		if err != nil {
			if strings.Contains(err.Error(), "model not found") {
				return c.Status(404).JSON(fiber.Map{"error": "Model not found"})
			}
			if strings.Contains(err.Error(), "model requires more system memory") {
				return c.Status(507).JSON(fiber.Map{"error": "Model requires more system memory"})
			}
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		return nil
	}
}