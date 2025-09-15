package main

import (
	"encoding/json"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type JobStatus string

const (
	JobPending   JobStatus = "pending"
	JobRunning   JobStatus = "running"
	JobFulfilled JobStatus = "fulfilled"
	JobFailed    JobStatus = "failed"
)

type JobType string

const (
	JobTypeGenerate   JobType = "generate"
	JobTypeOCRExtract JobType = "ocr_extract"
)

type Job struct {
	ID          string     `json:"id" gorm:"primaryKey"`
	CreatedAt   time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
	FulfilledAt *time.Time `json:"fulfilled_at,omitempty"`
	Status      JobStatus  `json:"status" gorm:"not null"`
	Model       string     `json:"model" gorm:"not null"`
	JobType     JobType    `json:"job_type" gorm:"not null"`
	Prompt      string     `json:"prompt,omitempty"`
	Result      string     `json:"result,omitempty"`
	ImagesPath  string     `json:"-" gorm:"column:images_path"` // Store as JSON string in DB
}

// GetImagesPathSlice returns ImagesPath as a slice
func (j *Job) GetImagesPathSlice() []string {
	if j.ImagesPath == "" {
		return []string{}
	}
	var paths []string
	json.Unmarshal([]byte(j.ImagesPath), &paths)
	return paths
}

// SetImagesPathSlice sets ImagesPath from a slice
func (j *Job) SetImagesPathSlice(paths []string) {
	if len(paths) == 0 {
		j.ImagesPath = ""
		return
	}
	data, _ := json.Marshal(paths)
	j.ImagesPath = string(data)
}

func CreateGenerationJob(request GenerationRequest) (*Job, error) {
	// Create the Job object
	job := &Job{
		ID:      uuid.New().String(),
		Status:  JobPending,
		JobType: JobTypeGenerate,
		Prompt:  request.Prompt,
		Model:   request.Model,
	}

	log.Printf("Creating generation job: ID=%s, Model=%s, Prompt=%s", job.ID, job.Model, job.Prompt)

	// Save the job to the database
	db := GetDB()
	if err := db.Create(job).Error; err != nil {
		log.Printf("Failed to create generation job: ID=%s, error=%v", job.ID, err)
		return nil, err
	}

	log.Printf("Generation job created successfully: ID=%s", job.ID)
	return job, nil
}

func CreateMultimodalExtractionJob(request MultiModalExtractionRequest) (*Job, error) {
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
	job := &Job{
		ID:      id,
		Status:  JobPending,
		JobType: JobTypeOCRExtract,
		Model:   request.Model,
		Prompt:  extraction_prompt,
	}

	// Set the images path using the helper method
	job.SetImagesPathSlice([]string{filePath})

	// Save the job to the database
	db := GetDB()
	if err := db.Create(job).Error; err != nil {
		log.Printf("Failed to create multimodal extraction job: ID=%s, error=%v", job.ID, err)
		return nil, err
	}

	log.Printf("Multimodal extraction job created successfully: ID=%s, FilePath=%s", job.ID, filePath)
	return job, nil
}

func GetJobStatus(id string) (*JobStatus, error) {
	db := GetDB()
	var job Job

	err := db.Select("status").Where("id = ?", id).First(&job).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // Job not found error
		}
		return nil, err // Other errors
	}

	return &job.Status, nil
}

func GetJob(id string, withResult bool) (*Job, error) {
	db := GetDB()
	var job Job

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

func UpdateJobResult(id string, status JobStatus, result string) error {
	db := GetDB()
	now := time.Now()
	return db.Model(&Job{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":       status,
		"result":       result,
		"fulfilled_at": &now,
	}).Error
}

func UpdateJobStatus(id string, status JobStatus) error {
	db := GetDB()
	return db.Model(&Job{}).Where("id = ?", id).Update("status", status).Error
}

func ListJobs(limit int, withResult bool) ([]Job, error) {
	db := GetDB()
	var jobs []Job

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

// Worker: polls for pending jobs and processes them
func StartJobWorker() {
	go func() {
		interval := 5
		if v := os.Getenv("JOB_WORKER_INTERVAL_SECONDS"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				interval = n
			}
		}
		for {
			processPendingJobs()
			time.Sleep(time.Duration(interval) * time.Second)
		}
	}()
}

func processPendingJobs() {
	// Fetch pending jobs from the database
	db := GetDB()
	var jobs []Job
	err := db.Where("status = ?", JobPending).Find(&jobs).Error
	if err != nil {
		log.Printf("Error querying pending jobs: %v", err)
		return
	}

	// Iterate through the jobs and process each one
	for _, job := range jobs {
		log.Printf("Processing job: ID=%s, Type=%s, Model=%s", job.ID, job.JobType, job.Model)

		// Update job status to running
		UpdateJobStatus(job.ID, JobRunning)

		var result string
		var status JobStatus
		switch job.JobType {
		case JobTypeGenerate: // Handle generation jobs
			var req GenerationRequest
			url := GetOllamaURL()

			req.Prompt = job.Prompt
			req.Model = job.Model

			resp, err := GenerateResponse(url, req)
			if err != nil {
				result = err.Error()
				status = JobFailed
				log.Printf("Job %s failed (generation): %v", job.ID, err)
			} else {
				if jsonBytes, err := json.Marshal(resp); err == nil {
					result = string(jsonBytes)
					status = JobFulfilled
					log.Printf("Job %s fulfilled (generation)", job.ID)
				} else {
					result = err.Error()
					status = JobFailed
					log.Printf("Job %s failed to marshal response: %v", job.ID, err)
				}
			}
		case JobTypeOCRExtract: // Handle MultiModal Extraction jobs
			url := GetOllamaURL()
			imagesPath := job.GetImagesPathSlice()

			// Read the file bytes from the path
			if len(imagesPath) > 0 {
				filePath := imagesPath[0]
				fileBytes, err := os.ReadFile(filePath)
				if err != nil {
					result = err.Error()
					status = JobFailed
					log.Printf("Job %s failed to read image: %v", job.ID, err)
				} else {
					// Extract filename from filePath for reporting
					var filename string
					if idx := len(filePath) - 1; idx >= 0 {
						for i := len(filePath) - 1; i >= 0; i-- {
							if filePath[i] == '/' || filePath[i] == '\\' {
								filename = filePath[i+1:]
								break
							}
						}
						if filename == "" {
							filename = filePath
						}
					} else {
						filename = filePath
					}
					resp, err_ := MultiModalTextExtractionFromImage(url, job.Model, fileBytes, filename)
					if err_ != nil {
						result = err_.Error()
						status = JobFailed
						log.Printf("Job %s failed (OCR extraction): %v", job.ID, err_)
					} else {
						if jsonBytes, err := json.Marshal(resp); err == nil {
							result = string(jsonBytes)
							status = JobFulfilled
							log.Printf("Job %s fulfilled (OCR extraction)", job.ID)
						} else {
							result = err.Error()
							status = JobFailed
							log.Printf("Job %s failed to marshal OCR response: %v", job.ID, err)
						}
					}
					// Clean up the temporary file
					os.Remove(filePath)
				}
			} else {
				result = "No image path found"
				status = JobFailed
				log.Printf("Job %s failed: no image path found", job.ID)
			}
		default: // Handle unknown job types
			result = ""
			status = JobFailed
			log.Printf("Job %s failed: unknown job type %s", job.ID, job.JobType)
		}

		// Update job result and status in the database
		UpdateJobResult(job.ID, status, result)
		log.Printf("Job %s updated with status %s", job.ID, status)
	}
}

// Helper to get OLLAMA_URL from env
func GetOllamaURL() string {
	url := os.Getenv("OLLAMA_URL")
	return url
}

// Check if job result is still retrievable based on the expiration time
// The default expiration time is 60 minutes, but can be overridden by the JOB_RESULT_EXPIRY_MINUTES environment variable
func IsJobResultRetrievable(job *Job) bool {
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

// Admin-only: Delete all jobs from the database
func EmptyJobs() error {
	return DeleteAllJobs()
}
