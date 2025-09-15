package jobs

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"zllm/internal/database"
	"zllm/internal/models"
)

// CreateGenerationJob creates a new text generation job
func CreateGenerationJob(request GenerationRequest) (*models.Job, error) {
	// Create the Job object
	job := &models.Job{
		ID:      uuid.New().String(),
		Status:  models.JobPending,
		JobType: models.JobTypeGenerate,
		Prompt:  request.Prompt,
		Model:   request.Model,
	}

	log.Printf("Creating generation job: ID=%s, Model=%s, Prompt=%s", job.ID, job.Model, job.Prompt)

	// Save the job to the database
	db := database.GetDB()
	if err := db.Create(job).Error; err != nil {
		log.Printf("Failed to create generation job: ID=%s, error=%v", job.ID, err)
		return nil, err
	}

	log.Printf("Generation job created successfully: ID=%s", job.ID)
	return job, nil
}

// CreateMultimodalExtractionJob creates a new multimodal text extraction job
func CreateMultimodalExtractionJob(request MultiModalExtractionRequest) (*models.Job, error) {
	id := uuid.New().String()

	log.Printf("Creating multimodal extraction job: ID=%s, Model=%s, FileExtension=%s", id, request.Model, request.FileExtension)

	// Ensure the /tmp-files directory exists
	tmpDir := "/tmp-files"
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		log.Printf("Failed to create tmp directory for job: ID=%s, error=%v", id, err)
		return nil, err
	}

	// Save the file bytes to a temporary location with the "{ID}.extension" as the filename
	filePath := tmpDir + "/" + id + request.FileExtension
	err := os.WriteFile(filePath, request.FileBytes, 0644)
	if err != nil {
		log.Printf("Failed to write file for multimodal extraction job: ID=%s, error=%v", id, err)
		return nil, err
	}

	// TODO: Improve the prompt for extraction
	extraction_prompt := "Please carefully extract and transcribe all text visible in this image. Return your response as a JSON object with the following structure: {\"original_text\": \"[extracted text]\"}"

	// Create the Job object
	job := &models.Job{
		ID:      id,
		Status:  models.JobPending,
		JobType: models.JobTypeOCRExtract,
		Model:   request.Model,
		Prompt:  extraction_prompt,
	}

	// Set the images path using the helper method
	job.SetImagesPathSlice([]string{filePath})

	// Save the job to the database
	db := database.GetDB()
	if err := db.Create(job).Error; err != nil {
		log.Printf("Failed to create multimodal extraction job: ID=%s, error=%v", job.ID, err)
		return nil, err
	}

	log.Printf("Multimodal extraction job created successfully: ID=%s, FilePath=%s", job.ID, filePath)
	return job, nil
}

// GetJobStatus returns the status of a job by ID
func GetJobStatus(id string) (*models.JobStatus, error) {
	db := database.GetDB()
	var job models.Job

	err := db.Select("status").Where("id = ?", id).First(&job).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // Job not found error
		}
		return nil, err // Other errors
	}

	return &job.Status, nil
}

// GetJob retrieves a job by ID
func GetJob(id string, withResult bool) (*models.Job, error) {
	db := database.GetDB()
	var job models.Job

	query := db.Where("id = ?", id)
	if !withResult {
		query = query.Omit("result")
	}

	err := query.First(&job).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, err
		}
		return nil, err
	}

	return &job, nil
}

// UpdateJobResult updates job status and result
func UpdateJobResult(id string, status models.JobStatus, result string) error {
	db := database.GetDB()
	now := time.Now()
	return db.Model(&models.Job{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":       status,
		"result":       result,
		"fulfilled_at": &now,
	}).Error
}

// UpdateJobStatus updates only the job status
func UpdateJobStatus(id string, status models.JobStatus) error {
	db := database.GetDB()
	return db.Model(&models.Job{}).Where("id = ?", id).Update("status", status).Error
}

// ListJobs returns a list of jobs
func ListJobs(limit int, withResult bool) ([]models.Job, error) {
	db := database.GetDB()
	var jobs []models.Job

	query := db.Order("created_at DESC").Limit(limit)
	if !withResult {
		query = query.Omit("result")
	}

	err := query.Find(&jobs).Error
	if err != nil {
		return nil, err
	}

	return jobs, nil
}

// IsJobResultRetrievable checks if job result is still retrievable based on expiration time
func IsJobResultRetrievable(job *models.Job) bool {
	if job.FulfilledAt == nil {
		return false
	}
	expiryMinutes := 60
	if v := os.Getenv("JOB_RESULT_EXPIRY_MINUTES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			expiryMinutes = n
		}
	}
	return time.Since(*job.FulfilledAt) < time.Duration(expiryMinutes)*time.Minute
}

// EmptyJobs deletes all jobs from the database
func EmptyJobs() error {
	return database.DeleteAllJobs()
}