package main

import (
	"bufio"
	"io"
	"log"
	"os"
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
		result, err := ExtractTextFromImage(url, modelName, fileBytes, file.Filename)
		if err != nil {
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
		var req GenerateRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
		}
		if req.Prompt == "" || req.Model == "" {
			return c.Status(400).JSON(fiber.Map{"error": "Prompt and model are required"})
		}
		job, err := CreateJob(JobTypeGenerate, req)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"job_id": job.ID, "status": job.Status})
	})

	// POST job/multimodal/extract/image
	// Create a new asynchronous job to extract text from an image
	// This endpoint requires a JWT token for authentication
	app.Post("/job/multimodal/extract/image", JWTMiddleware(), func(c *fiber.Ctx) error {
		modelName := c.Query("model", "")
		if modelName == "" {
			return c.Status(400).JSON(fiber.Map{
				"error":            "Model parameter is required. Specify one of the supported multimodal models.",
				"supported_models": []MultimodalModel{Gemma3, Llava7, MiniCPMv},
			})
		}
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
		file, err := c.FormFile("file")
		if err != nil {
			return c.Status(400).JSON(fiber.Map{
				"error":   "Error parsing file",
				"details": err.Error(),
			})
		}
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
		fileContent, err := file.Open()
		if err != nil {
			return c.Status(500).JSON(fiber.Map{
				"error":   "Error opening file",
				"details": err.Error(),
			})
		}
		defer fileContent.Close()
		fileBytes, err := io.ReadAll(fileContent)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{
				"error":   "Error reading file content",
				"details": err.Error(),
			})
		}
		input := struct {
			ModelName string `json:"model"`
			FileBytes []byte `json:"file_bytes"`
			Filename  string `json:"filename"`
		}{
			ModelName: modelName,
			FileBytes: fileBytes,
			Filename:  file.Filename,
		}
		job, err := CreateJob(JobTypeOCRExtract, input)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"job_id": job.ID, "status": job.Status})
	})

	// GET job/:id/status
	// Check asynchronous job status
	// This endpoint requires a JWT token for authentication
	app.Get("/job/:id/status", JWTMiddleware(), func(c *fiber.Ctx) error {
		id := c.Params("id")
		job, err := GetJob(id)
		if err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "Job not found"})
		}
		return c.JSON(fiber.Map{
			"job_id":       job.ID,
			"status":       job.Status,
			"job_type":     job.JobType,
			"created_at":   job.CreatedAt,
			"fulfilled_at": job.FulfilledAt,
		})
	})

	// GET job/:id/result
	// Retrieve asynchronous job result
	// This endpoint requires a JWT token for authentication
	app.Get("/job/:id/result", JWTMiddleware(), func(c *fiber.Ctx) error {
		id := c.Params("id")
		job, err := GetJob(id)
		if err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "Job not found"})
		}
		if job.Status != JobFulfilled {
			return c.Status(202).JSON(fiber.Map{"status": job.Status, "message": "Job not fulfilled yet"})
		}
		if !IsJobResultRetrievable(job) {
			return c.Status(410).JSON(fiber.Map{"error": "Result expired"})
		}
		return c.JSON(fiber.Map{
			"job_id": job.ID,
			"result": job.Result,
		})
	})

	// GET job/list
	// List the last previous jobs
	// This endpoint requires a JWT token for authentication and ADMIN role
	app.Get("/job/list", JWTMiddleware(), AdminMiddleware(), func(c *fiber.Ctx) error {
		limit := 10
		if l := c.Query("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil && n > 0 {
				limit = n
			}
		}
		includeResult := false
		if r := c.Query("result"); r != "" {
			includeResult = strings.ToLower(r) == "true"
		}
		jobs, err := ListJobs(limit, includeResult)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		// If result=false, remove the Result field from each job before returning
		if !includeResult {
			type JobNoResult struct {
				ID          string       `json:"id"`
				CreatedAt   interface{}  `json:"created_at"`
				FulfilledAt *interface{} `json:"fulfilled_at,omitempty"`
				Status      JobStatus    `json:"status"`
				JobType     JobType      `json:"job_type"`
				Input       string       `json:"input"`
			}
			jobsNoResult := make([]JobNoResult, 0, len(jobs))
			for _, job := range jobs {
				createdAt := job.CreatedAt
				var fulfilledAt interface{}
				if job.FulfilledAt != nil {
					fulfilledAt = *job.FulfilledAt
				} else {
					fulfilledAt = nil
				}
				jobsNoResult = append(jobsNoResult, JobNoResult{
					ID:          job.ID,
					CreatedAt:   createdAt,
					FulfilledAt: &fulfilledAt,
					Status:      job.Status,
					JobType:     job.JobType,
					Input:       job.Input,
				})
			}
			return c.JSON(fiber.Map{"jobs": jobsNoResult})
		}
		return c.JSON(fiber.Map{"jobs": jobs})
	})

	log.Fatal(app.Listen(":3000"))
}
