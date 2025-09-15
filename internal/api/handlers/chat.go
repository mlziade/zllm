package handlers

import (
	"bufio"
	"strings"

	"github.com/gofiber/fiber/v2"

	"zllm/internal/ollama"
)

// HandleChat processes chat requests
func HandleChat(client *ollama.Client) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Parse the request body
		var req ollama.ChatRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).SendString("Error parsing request body")
		}

		// Validate required fields
		if req.Model == "" {
			return c.Status(400).SendString("Model is required")
		}
		if len(req.Messages) == 0 {
			return c.Status(400).SendString("Messages are required")
		}

		// Generate the chat response
		response, err := client.ChatResponse(req)
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

// HandleChatStream processes streaming chat requests
func HandleChatStream(client *ollama.Client) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Parse the request body
		var req ollama.ChatRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).SendString("Error parsing request body")
		}

		// Validate required fields
		if req.Model == "" {
			return c.Status(400).SendString("Model is required")
		}
		if len(req.Messages) == 0 {
			return c.Status(400).SendString("Messages are required")
		}

		// Set headers for streaming
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")

		// Create a buffered writer for streaming
		writer := bufio.NewWriter(c.Response().BodyWriter())

		// Stream the chat response
		err := client.StreamChatResponse(req, writer)
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