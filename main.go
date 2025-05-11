package main

import (
	"bufio"
	"io"
	"log"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
)

/* Ollama Server API endpoints
 *
 * https://github.com/ollama/ollama/blob/main/docs/api.md
 *
 */

func main() {
	// Create a new Fiber server
	app := fiber.New()

	// Authentication endpoint
	app.Post("/auth", func(c *fiber.Ctx) error {
		// Load the .env file
		if err := godotenv.Load(); err != nil {
			log.Println("Warning: Error loading .env file")
		}

		// Get API keys from .env
		normalAPIKey := os.Getenv("API_KEY")
		adminAPIKey := os.Getenv("ADMIN_API_KEY")

		if normalAPIKey == "" || adminAPIKey == "" {
			return c.Status(500).JSON(fiber.Map{"error": "API keys are not properly set in the .env file"})
		}

		// Get JWT secret key from .env
		jwtSecret := os.Getenv("JWT_SECRET")
		if jwtSecret == "" {
			return c.Status(500).JSON(fiber.Map{"error": "JWT_SECRET is not set in the .env file"})
		}

		// Parse the request body
		var req AuthRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Error parsing request body"})
		}

		// Process the authentication request
		tokenString, role, expirationTime, err := HandleAuthentication(normalAPIKey, adminAPIKey, jwtSecret, req)
		if err != nil {
			return c.Status(401).JSON(fiber.Map{"error": err.Error()})
		}

		// Return the token, role, and expiration time
		return c.JSON(fiber.Map{
			"token":      tokenString,
			"role":       role,
			"expires_at": expirationTime,
		})
	})

	// GET llms/models/list
	// List all models available locally on Ollama
	app.Get("/llms/models/list", JWTMiddleware(), func(c *fiber.Ctx) error {
		// Load the .env file
		if err := godotenv.Load(); err != nil {
			log.Println("Warning: Error loading .env file")
		}

		// Get the Ollama URL from the .env file
		url := os.Getenv("OLLAMA_URL")
		if url == "" {
			return c.Status(500).SendString("OLLAMA_URL is not set in the .env file")
		}

		// Get the list of models from Ollama
		models, err := ListModels(url)
		if err != nil {
			return c.Status(500).SendString(err.Error())
		}

		// Return the formatted response
		return c.JSON(fiber.Map{"models": models})
	})

	// POST llms/generate
	// Generate a answer from a model and prompt
	app.Post("/llms/generate", JWTMiddleware(), func(c *fiber.Ctx) error {
		// Load the .env file
		if err := godotenv.Load(); err != nil {
			log.Println("Warning: Error loading .env file")
		}

		// Get the Ollama URL from the .env file
		url := os.Getenv("OLLAMA_URL")
		if url == "" {
			return c.Status(500).SendString("OLLAMA_URL is not set in the .env file")
		}

		// Parse the request body into an GenerateRequest structure
		var req GenerateRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).SendString("Error parsing request body")
		}

		// Ensure the prompt and model fields are provided
		if req.Prompt == "" || req.Model == "" {
			return c.Status(400).SendString("Prompt and model are required")
		}

		// Generate the response
		response, err := GenerateResponse(url, req)
		if err != nil {
			return c.Status(500).SendString(err.Error())
		}

		return c.JSON(response)
	})

	// POST llms/generate-streaming
	// Generate an answer from a model and prompt with a streaming response
	app.Post("/llms/generate-streaming", JWTMiddleware(), func(c *fiber.Ctx) error {
		// Load the .env file
		if err := godotenv.Load(); err != nil {
			log.Println("Warning: Error loading .env file")
		}

		// Get the Ollama URL from the .env file
		url := os.Getenv("OLLAMA_URL")
		if url == "" {
			return c.Status(500).SendString("OLLAMA_URL is not set in the .env file")
		}

		// Parse the request body into a GenerateRequest structure
		var req GenerateRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).SendString("Error parsing request body")
		}

		// Ensure the prompt and model fields are provided
		if req.Prompt == "" || req.Model == "" {
			return c.Status(400).SendString("Prompt and model are required")
		}

		// Set response headers for streaming
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		c.Set("Transfer-Encoding", "chunked")

		// Stream the response from Ollama to the client
		c.Response().SetBodyStreamWriter(func(w *bufio.Writer) {
			if err := StreamResponse(url, req, w); err != nil {
				log.Printf("Error streaming response: %v", err)
			}
		})
		return nil
	})

	// POST llms/models/add
	// Pull a model from the Ollama library
	app.Post("/llms/models/add", JWTMiddleware(), AdminMiddleware(), func(c *fiber.Ctx) error {
		// Load the .env file
		if err := godotenv.Load(); err != nil {
			log.Println("Warning: Error loading .env file")
		}

		// Get the Ollama URL from the .env file
		url := os.Getenv("OLLAMA_URL")
		if url == "" {
			return c.Status(500).SendString("OLLAMA_URL is not set in the .env file")
		}

		// Parse the request body into an AddModelRequest structure
		var req AddModelRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).SendString("Error parsing request body")
		}

		// Ensure the model field is provided
		if req.Model == "" {
			return c.Status(400).SendString("Model is required")
		}

		// Add the model
		response, err := AddModel(url, req)
		if err != nil {
			return c.Status(500).SendString(err.Error())
		}

		// Only return a 200 if the status is "success"
		if status, ok := response["status"].(string); ok && status == "success" {
			return c.Status(200).JSON(fiber.Map{"status": "success"})
		}

		// Return appropriate status code with the error from Ollama
		return c.Status(400).JSON(response)
	})

	// DELETE llms/models/delete
	// Delete a model from Ollama
	app.Delete("/llms/models/delete", JWTMiddleware(), AdminMiddleware(), func(c *fiber.Ctx) error {
		// Load the .env file
		if err := godotenv.Load(); err != nil {
			log.Println("Warning: Error loading .env file")
		}

		// Get the Ollama URL from the .env file
		url := os.Getenv("OLLAMA_URL")
		if url == "" {
			return c.Status(500).SendString("OLLAMA_URL is not set in the .env file")
		}

		// Parse the request body into a DeleteModelRequest structure
		var req DeleteModelRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).SendString("Error parsing request body")
		}

		// Ensure the model field is provided
		if req.Model == "" {
			return c.Status(400).SendString("Model name is required")
		}

		// Delete the model
		err := DeleteModel(url, req)
		if err != nil {
			return c.Status(500).SendString(err.Error())
		}

		// Successful deletion
		return c.JSON(fiber.Map{"status": "success", "message": "Model deleted successfully"})
	})

	// POST ocr/extract/image
	// Extract text from an image using multimodal LLMs
	app.Post("ocr/extract/image", JWTMiddleware(), func(c *fiber.Ctx) error {
		// Load the .env file
		if err := godotenv.Load(); err != nil {
			log.Println("Warning: Error loading .env file")
		}

		// Get the Ollama URL from the .env file
		url := os.Getenv("OLLAMA_URL")
		if url == "" {
			return c.Status(500).SendString("OLLAMA_URL is not set in the .env file")
		}

		// Get the model parameter from query - no default value
		modelName := c.Query("model", "")

		// Return error if model parameter is not provided
		if modelName == "" {
			return c.Status(400).JSON(fiber.Map{
				"error":            "Model parameter is required. Specify one of the supported multimodal models.",
				"supported_models": []MultimodalModel{Gemma3, Llava7, MiniCPMv},
			})
		}

		// Validate the model is a supported multimodal model
		isValidModel := false
		supportedModels := []MultimodalModel{Gemma3, Llava7, MiniCPMv}
		for _, m := range supportedModels {
			if string(m) == modelName {
				isValidModel = true
				break
			}
		}

		if !isValidModel {
			return c.Status(400).JSON(fiber.Map{
				"error":            "Unsupported model. Use one of the supported multimodal models.",
				"supported_models": supportedModels,
			})
		}

		// Get the image file from the request
		file, err := c.FormFile("file")
		if err != nil {
			return c.Status(400).JSON(fiber.Map{
				"error":   "Error parsing file",
				"details": err.Error(),
			})
		}

		// Validate file extension (only allow .png, .jpg, .jpeg)
		filename := strings.ToLower(file.Filename)
		isValidExtension := false
		allowedExtensions := []string{".png", ".jpg", ".jpeg"}
		for _, ext := range allowedExtensions {
			if strings.HasSuffix(filename, ext) {
				isValidExtension = true
				break
			}
		}

		if !isValidExtension {
			return c.Status(400).JSON(fiber.Map{
				"error": "Unsupported file type. Only .png, .jpg, and .jpeg images are supported",
			})
		}

		// Open the uploaded file
		fileContent, err := file.Open()
		if err != nil {
			return c.Status(500).JSON(fiber.Map{
				"error":   "Error opening file",
				"details": err.Error(),
			})
		}
		defer fileContent.Close()

		// Read the file content
		fileBytes, err := io.ReadAll(fileContent)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{
				"error":   "Error reading file content",
				"details": err.Error(),
			})
		}

		// Process the image
		result, err := ExtractTextFromImage(url, modelName, fileBytes, file.Filename)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{
				"error":   "Error processing image",
				"details": err.Error(),
			})
		}

		return c.JSON(result)
	})

	log.Fatal(app.Listen(":3000"))
}
