package handlers

import (
	"github.com/gofiber/fiber/v2"

	"zllm/internal/ollama"
)

// HandleListModels lists all available models
func HandleListModels(client *ollama.Client) fiber.Handler {
	return func(c *fiber.Ctx) error {
		models, err := client.ListModels()
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(fiber.Map{"models": models})
	}
}

// HandleAddModel adds a new model
func HandleAddModel(client *ollama.Client) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Parse the request body
		var req ollama.AddModelRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).SendString("Error parsing request body")
		}

		// Validate required fields
		if req.Model == "" {
			return c.Status(400).SendString("Model is required")
		}

		// Add the model
		response, err := client.AddModel(req)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(response)
	}
}

// HandleDeleteModel deletes a model
func HandleDeleteModel(client *ollama.Client) fiber.Handler {
	return func(c *fiber.Ctx) error {
		model := c.Params("model")
		if model == "" {
			return c.Status(400).SendString("Model parameter is required")
		}

		req := ollama.DeleteModelRequest{Model: model}
		err := client.DeleteModel(req)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(fiber.Map{"message": "Model deleted successfully"})
	}
}