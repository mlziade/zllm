package main

import (
	"bufio"
	"io"
	"log"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
)

func main() {
	// Start the job worker so jobs are processed and visible
	StartJobWorker()

	// Create a new Fiber server
	app := fiber.New()

	/* Auth Endpoints */

	// Authentication endpoint
	app.Post("/auth", func(c *fiber.Ctx) error {
		// Load the .env file
		if err := godotenv.Load(); err != nil {
			log.Println("Warning: Error loading .env file")
		}

		// Get API keys from .env and check if they are set
		normalAPIKey := os.Getenv("API_KEY")
		adminAPIKey := os.Getenv("ADMIN_API_KEY")
		if normalAPIKey == "" || adminAPIKey == "" {
			return c.Status(500).JSON(fiber.Map{"error": "API keys are not properly set in the .env file"})
		}

		// Get JWT secret key from .env and check if it is set
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

	/* LLMs Endpoints */

	// POST llm/generate
	// Generate a answer from a model and prompt
	// This endpoint requires a JWT token for authentication
	app.Post("/llm/generate", JWTMiddleware(), func(c *fiber.Ctx) error {
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
		var req GenerationRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).SendString("Error parsing request body")
		}

		// Ensure the prompt and model fields are provided
		if req.Prompt == "" {
			return c.Status(400).SendString("Prompt is required")
		}
		if req.Model == "" {
			return c.Status(404).SendString("Model is required")
		}

		// Generate the response
		response, err := GenerateResponse(url, req)
		if err != nil {
			// Check for Ollama "model not found" error
			if strings.Contains(err.Error(), "model not found") {
				return c.Status(400).JSON(fiber.Map{"error": "model not found"})
			}
			return c.Status(500).SendString(err.Error())
		}

		return c.JSON(response)
	})

	// POST llm/generate/streaming
	// Generate an answer from a model and prompt with a streaming response
	// This endpoint requires a JWT token for authentication
	app.Post("/llm/generate/streaming", JWTMiddleware(), func(c *fiber.Ctx) error {
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
		var req GenerationRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).SendString("Error parsing request body")
		}

		// Ensure the prompt and model fields are provided
		if req.Prompt == "" {
			return c.Status(400).SendString("Prompt is required")
		}
		if req.Model == "" {
			return c.Status(404).SendString("Model is required")
		}

		// Set response headers for streaming
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		c.Set("Transfer-Encoding", "chunked")

		// Stream the response from Ollama to the client
		var streamErr error
		c.Response().SetBodyStreamWriter(func(w *bufio.Writer) {
			streamErr = StreamGenerationResponse(url, req, w)
			if streamErr != nil {
				log.Printf("Error streaming response: %v", streamErr)
			}
		})
		if streamErr != nil {
			// Check for Ollama "model not found" error
			if strings.Contains(streamErr.Error(), "model not found") {
				return c.Status(400).JSON(fiber.Map{"error": "model not found"})
			}
			return c.Status(500).SendString(streamErr.Error())
		}
		return nil
	})

	// POST llm/chat
	// Generate a chat response from a model and prompt
	// This endpoint requires a JWT token for authentication
	app.Post("/llm/chat", JWTMiddleware(), func(c *fiber.Ctx) error {
		// Load the .env file
		if err := godotenv.Load(); err != nil {
			log.Println("Warning: Error loading .env file")
		}

		// Get the Ollama URL from the .env file
		url := os.Getenv("OLLAMA_URL")
		if url == "" {
			log.Println("[llm/chat] OLLAMA_URL is not set in the .env file")
			return c.Status(500).SendString("OLLAMA_URL is not set in the .env file")
		}

		// Parse the request body into a ChatRequest structure
		var req ChatRequest
		if err := c.BodyParser(&req); err != nil {
			log.Printf("[llm/chat] Failed to parse request body: %v", err)
			return c.Status(400).SendString("Error parsing request body")
		}

		// Ensure the prompt and model fields are provided
		if req.Model == "" {
			log.Println("[llm/chat] Model is required in request")
			return c.Status(400).SendString("Model is required")
		}
		if len(req.Messages) == 0 {
			log.Println("[llm/chat] At least one message is required in request")
			return c.Status(400).SendString("At least one message is required")
		}

		log.Printf("[llm/chat] Received chat request | Model: %s | Messages: %+v", req.Model, req.Messages)

		// Generate the chat response
		response, err := ChatResponse(url, req)
		if err != nil {
			// Check for Ollama "model not found" error
			if strings.Contains(err.Error(), "model not found") {
				log.Printf("[llm/chat] Model not found: %s", req.Model)
				return c.Status(400).JSON(fiber.Map{"error": "model not found"})
			}
			log.Printf("[llm/chat] Failed to generate chat response: %v", err)
			return c.Status(500).SendString(err.Error())
		}

		log.Printf("[llm/chat] Chat response generated successfully | Model: %s", req.Model)
		// Append the assistant's response as the last message in the conversation
		fullConversation := append(req.Messages, Message{
			Role:    Assistant,
			Content: response["response"].(string),
		})

		// Add the full_conversation field to the response
		return c.JSON(fiber.Map{
			"model":             req.Model,
			"response":          response["response"],
			"full_conversation": fullConversation,
		})
	})

	// POST llm/chat/streaming
	// Generate a chat response from a model and prompt with a streaming response
	// This endpoint requires a JWT token for authentication
	app.Post("/llm/chat/streaming", JWTMiddleware(), func(c *fiber.Ctx) error {
		// Load the .env file
		if err := godotenv.Load(); err != nil {
			log.Println("Warning: Error loading .env file")
		}

		// Get the Ollama URL from the .env file
		url := os.Getenv("OLLAMA_URL")
		if url == "" {
			return c.Status(500).SendString("OLLAMA_URL is not set in the .env file")
		}

		// Parse the request body into a ChatRequest structure
		var req ChatRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).SendString("Error parsing request body")
		}

		// Ensure the model and messages fields are provided
		if req.Model == "" {
			return c.Status(400).SendString("Model is required")
		}
		if len(req.Messages) == 0 {
			return c.Status(400).SendString("At least one message is required")
		}

		// Set response headers for streaming
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		c.Set("Transfer-Encoding", "chunked")

		// Stream the response from Ollama to the client
		var streamErr error
		c.Response().SetBodyStreamWriter(func(w *bufio.Writer) {
			streamErr = StreamChatResponse(url, req, w)
			if streamErr != nil {
				log.Printf("Error streaming chat response: %v", streamErr)
			}
		})
		if streamErr != nil {
			if strings.Contains(streamErr.Error(), "model not found") {
				return c.Status(400).JSON(fiber.Map{"error": "model not found"})
			}
			return c.Status(500).SendString(streamErr.Error())
		}
		return nil
	})

	// POST llm/multimodal/extract/image
	// Extract text from an image using multimodal LLMs
	// This endpoint requires a JWT token for authentication
	app.Post("/llm/multimodal/extract/image", JWTMiddleware(), func(c *fiber.Ctx) error {
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
		result, err := MultiModalTextExtractionFromImage(url, modelName, fileBytes, file.Filename)
		if err != nil {
			// Check for Ollama "model not found" error
			if strings.Contains(err.Error(), "model not found") {
				return c.Status(400).JSON(fiber.Map{"error": "model not found"})
			}
			return c.Status(500).JSON(fiber.Map{
				"error":   "Error processing image",
				"details": err.Error(),
			})
		}

		return c.JSON(result)
	})

	// POST llm/model/add
	// Pull a model from the Ollama library and add it to the local instance
	// This endpoint requires a JWT token for authentication and ADMIN role
	app.Post("/llm/model/add", JWTMiddleware(), AdminMiddleware(), func(c *fiber.Ctx) error {
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

	// DELETE llm/model/delete
	// Delete a model from Ollama
	// This endpoint requires a JWT token for authentication and ADMIN role
	app.Delete("/llm/model/delete", JWTMiddleware(), AdminMiddleware(), func(c *fiber.Ctx) error {
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
			// Check if error message indicates model not found
			if strings.Contains(err.Error(), "model not found") {
				return c.Status(400).JSON(fiber.Map{"error": "model not found"})
			}
			return c.Status(500).SendString(err.Error())
		}

		// Successful deletion
		return c.JSON(fiber.Map{"status": "success", "message": "Model deleted successfully"})
	})

	// GET llm/model/list
	// List all models available locally on Ollama
	// This endpoint requires a JWT token for authentication
	app.Get("/llm/model/list", JWTMiddleware(), func(c *fiber.Ctx) error {
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

	/* Jobs Endpoints */

	// POST job/generate
	// Create a new asynchronous job to generate a response
	// This endpoint requires a JWT token for authentication
	app.Post("/job/generate", JWTMiddleware(), func(c *fiber.Ctx) error {
		var req GenerationRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
		}

		// Ensure the prompt and model fields are provided
		if req.Prompt == "" || req.Model == "" {
			return c.Status(400).JSON(fiber.Map{"error": "Prompt and model are required"})
		}

		// Create a new job
		job, err := CreateGenerationJob(req)
		if err != nil {
			// Check for Ollama "model not found" error
			if strings.Contains(err.Error(), "model not found") {
				return c.Status(400).JSON(fiber.Map{"error": "model not found"})
			}
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		// Return the job ID and status
		return c.JSON(fiber.Map{"job_id": job.ID, "status": job.Status})
	})

	// POST job/multimodal/extract/image
	// Create a new asynchronous job to extract text from an image
	// This endpoint requires a JWT token for authentication
	app.Post("/job/multimodal/extract/image", JWTMiddleware(), func(c *fiber.Ctx) error {
		var req MultiModalExtractionRequest
		// Check if the model is provided
		modelName := c.Query("model", "")
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

		filename := strings.ToLower(file.Filename)

		// Extract the file extension
		parts := strings.Split(strings.ToLower(filename), ".")
		var fileExtansion string
		if len(parts) > 1 {
			fileExtansion = "." + parts[len(parts)-1]
		} else {
			return c.Status(400).JSON(fiber.Map{
				"error": "Invalid file name. No extension found.",
			})
		}

		// Validate file extension (only allow .png, .jpg, .jpeg)
		allowedExtensions := []string{".png", ".jpg", ".jpeg"}
		if !slices.Contains(allowedExtensions, fileExtansion) {
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

		// Fill the MultiModalExtractionRequest
		req.Model = modelName
		req.FileBytes = fileBytes
		req.Filename = file.Filename
		req.FileExtension = fileExtansion

		// Create a new job
		job, err := CreateMultimodalExtractionJob(req)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		// Return the job ID and status
		return c.JSON(fiber.Map{"job_id": job.ID, "status": job.Status})
	})

	// GET job/list
	// List the last previous jobs
	// This endpoint requires a JWT token for authentication and ADMIN role
	app.Get("/job/list", JWTMiddleware(), AdminMiddleware(), func(c *fiber.Ctx) error {
		// Get the limit or defaults to 10
		limit := 10
		if l := c.Query("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil && n > 0 {
				limit = n
			}
		}

		// Get if the result should be included
		includeResult := false
		if r := c.Query("result"); r != "" {
			includeResult = strings.ToLower(r) == "true"
		}

		// Get the list of jobs
		jobs, err := ListJobs(limit, includeResult)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		// Always return an array for "jobs"
		if jobs == nil {
			jobs = []Job{}
		}

		// Return the list of jobs
		return c.JSON(fiber.Map{"jobs": jobs})
	})

	// GET job/:id/status
	// Check asynchronous job status
	// This endpoint requires a JWT token for authentication
	app.Get("/job/:id/status", JWTMiddleware(), func(c *fiber.Ctx) error {
		id := c.Params("id")
		statusPtr, err := GetJobStatus(id)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		if statusPtr == nil {
			return c.Status(404).JSON(fiber.Map{"error": "Job not found"})
		}
		return c.JSON(fiber.Map{
			"job_id": id,
			"status": *statusPtr,
		})
	})

	// GET job/:id/
	// Retrieve asynchronous job and its result, if available
	// This endpoint requires a JWT token for authentication
	app.Get("/job/:id", JWTMiddleware(), func(c *fiber.Ctx) error {
		id := c.Params("id")
		result := c.Query("result")

		withResult := result == "true"
		job, err := GetJob(id, withResult)
		if err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "Job not found"})
		}

		is_retrievable := IsJobResultRetrievable(job)

		// If the user requested the result, check if it's retrievable
		if withResult && !is_retrievable {
			return c.Status(410).JSON(fiber.Map{"error": "Result expired"})
		}

		// Build the response
		resp := fiber.Map{
			"id":           job.ID,
			"status":       job.Status,
			"created_at":   job.CreatedAt,
			"fulfilled_at": job.FulfilledAt,
			"job_type":     job.JobType,
			"prompt":       job.Prompt,
			"model":        job.Model,
		}
		if withResult && is_retrievable {
			resp["result"] = job.Result
		}

		return c.JSON(resp)
	})

	// DELETE job/empty
	// Admin-only endpoint to delete all jobs
	app.Delete("/job/empty", JWTMiddleware(), AdminMiddleware(), func(c *fiber.Ctx) error {
		err := EmptyJobs()
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"status": "success", "message": "All jobs deleted"})
	})

	log.Fatal(app.Listen(":3000"))
}
