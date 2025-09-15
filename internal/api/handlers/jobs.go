package handlers

import (
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"zllm/internal/jobs"
	"zllm/internal/models"
)

// Supported multimodal models
var supportedMultimodalModels = []string{"gemma3:4b", "llava:7b", "minicpm-v:8b"}

// HandleCreateGenerationJob creates a new text generation job
func HandleCreateGenerationJob() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Parse the request body
		var req jobs.GenerationRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).SendString("Error parsing request body")
		}

		// Validate required fields
		if req.Prompt == "" {
			return c.Status(400).SendString("Prompt is required")
		}
		if req.Model == "" {
			return c.Status(400).SendString("Model is required")
		}

		// Create the job
		job, err := jobs.CreateGenerationJob(req)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		return c.Status(201).JSON(fiber.Map{
			"id":      job.ID,
			"status":  job.Status,
			"message": "Generation job created successfully",
		})
	}
}

// HandleCreateMultimodalJob creates a new multimodal extraction job
func HandleCreateMultimodalJob() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Parse multipart form
		form, err := c.MultipartForm()
		if err != nil {
			return c.Status(400).SendString("Error parsing multipart form")
		}

		// Get model from form
		models := form.Value["model"]
		if len(models) == 0 || models[0] == "" {
			return c.Status(400).SendString("Model is required")
		}
		model := models[0]

		// Validate model
		if !slices.Contains(supportedMultimodalModels, model) {
			return c.Status(400).JSON(fiber.Map{
				"error":             "Unsupported model for multimodal extraction",
				"supported_models": supportedMultimodalModels,
			})
		}

		// Get file from form
		files := form.File["file"]
		if len(files) == 0 {
			return c.Status(400).SendString("File is required")
		}
		file := files[0]

		// Read file content
		fileContent, err := file.Open()
		if err != nil {
			return c.Status(500).SendString("Error opening uploaded file")
		}
		defer fileContent.Close()

		fileBytes, err := io.ReadAll(fileContent)
		if err != nil {
			return c.Status(500).SendString("Error reading uploaded file")
		}

		// Get file extension
		filename := file.Filename
		extensionIndex := strings.LastIndex(filename, ".")
		var fileExtension string
		if extensionIndex != -1 {
			fileExtension = filename[extensionIndex:]
		} else {
			fileExtension = ".unknown"
		}

		// Create request
		req := jobs.MultiModalExtractionRequest{
			Model:         model,
			FileBytes:     fileBytes,
			FileExtension: fileExtension,
		}

		// Create the job
		job, err := jobs.CreateMultimodalExtractionJob(req)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		return c.Status(201).JSON(fiber.Map{
			"id":      job.ID,
			"status":  job.Status,
			"message": "Multimodal extraction job created successfully",
		})
	}
}

// HandleGetJobStatus returns the status of a job
func HandleGetJobStatus() fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		if id == "" {
			return c.Status(400).SendString("Job ID is required")
		}

		status, err := jobs.GetJobStatus(id)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		if status == nil {
			return c.Status(404).JSON(fiber.Map{"error": "Job not found"})
		}

		return c.JSON(fiber.Map{"status": *status})
	}
}

// HandleGetJobResult returns the result of a job
func HandleGetJobResult() fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		if id == "" {
			return c.Status(400).SendString("Job ID is required")
		}

		job, err := jobs.GetJob(id, true)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				return c.Status(404).JSON(fiber.Map{"error": "Job not found"})
			}
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		// Check if job is fulfilled and result is still retrievable
		if job.Status != models.JobFulfilled {
			return c.Status(200).JSON(fiber.Map{
				"id":     job.ID,
				"status": job.Status,
			})
		}

		if !jobs.IsJobResultRetrievable(job) {
			return c.Status(410).JSON(fiber.Map{"error": "Job result has expired"})
		}

		return c.JSON(fiber.Map{
			"id":     job.ID,
			"status": job.Status,
			"result": job.Result,
		})
	}
}

// HandleListJobs returns a list of jobs (admin only)
func HandleListJobs() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Parse limit parameter
		limit := 50 // default limit
		if limitStr := c.Query("limit"); limitStr != "" {
			if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
				limit = parsedLimit
			}
		}

		// Parse with_result parameter
		withResult := c.Query("with_result") == "true"

		jobsList, err := jobs.ListJobs(limit, withResult)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(fiber.Map{"jobs": jobsList})
	}
}

// HandleDeleteAllJobs deletes all jobs (admin only)
func HandleDeleteAllJobs() fiber.Handler {
	return func(c *fiber.Ctx) error {
		err := jobs.EmptyJobs()
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(fiber.Map{"message": "All jobs deleted successfully"})
	}
}