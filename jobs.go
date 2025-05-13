package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
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
	ID          string     `json:"id"`
	CreatedAt   time.Time  `json:"created_at"`
	FulfilledAt *time.Time `json:"fulfilled_at,omitempty"`
	Status      JobStatus  `json:"status"`
	Model       string     `json:"model"`
	JobType     JobType    `json:"job_type"`
	Prompt      string     `json:"prompt,omitempty"`
	Result      string     `json:"result,omitempty"`
	ImagesPath  []string   `json:"images_path,omitempty"`
}

func CreateGenerationJob(request GenerationRequest) (*Job, error) {
	// Generate a unique ID for the job
	id := uuid.New().String()
	now := time.Now()

	// Create the Job object
	job := &Job{
		ID:        id,
		CreatedAt: now,
		Status:    JobPending,
		JobType:   JobTypeGenerate,
		Prompt:    request.Prompt,
		Model:     request.Model,
	}

	log.Printf("Creating generation job: ID=%s, Model=%s, Prompt=%s", job.ID, job.Model, job.Prompt)

	// Save the job to the database
	db := GetDB()
	_, err := db.Exec(
		`INSERT INTO jobs (id, created_at, status, job_type, prompt, model) VALUES (?, ?, ?, ?, ?, ?)`,
		job.ID, job.CreatedAt, job.Status, job.JobType, job.Prompt, job.Model,
	)
	if err != nil {
		log.Printf("Failed to create generation job: ID=%s, error=%v", job.ID, err)
		return nil, err
	}

	log.Printf("Generation job created successfully: ID=%s", job.ID)
	return job, nil
}

func CreateMultimodalExtractionJob(request MultiModalExtractionRequest) (*Job, error) {
	// Generate a unique ID for the job
	id := uuid.New().String()
	now := time.Now()

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

	// Create the Job object
	job := &Job{
		ID:         id,
		CreatedAt:  now,
		Status:     JobPending,
		JobType:    JobTypeOCRExtract,
		ImagesPath: []string{filePath},
		Model:      request.Model,
	}

	// Marshal ImagesPath to JSON for DB storage
	imagesPathJSON, err := json.Marshal(job.ImagesPath)
	if err != nil {
		log.Printf("Failed to marshal images path for job: ID=%s, error=%v", id, err)
		return nil, err
	}

	// TODO: Improve the prompt for extraction
	extraction_prompt := "Please carefully extract and transcribe all text visible in this image. Return your response as a JSON object with the following structure: {\"original_text\": \"[extracted text]\"}"

	// Save the job to the database
	db := GetDB()
	_, err = db.Exec(
		`INSERT INTO jobs (id, created_at, status, job_type, prompt, images_path, model) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		job.ID, job.CreatedAt, job.Status, job.JobType, extraction_prompt, string(imagesPathJSON), job.Model,
	)
	if err != nil {
		log.Printf("Failed to create multimodal extraction job: ID=%s, error=%v", job.ID, err)
		return nil, err
	}

	log.Printf("Multimodal extraction job created successfully: ID=%s, FilePath=%s", job.ID, filePath)
	return job, nil
}

func GetJobStatus(id string) (*JobStatus, error) {
	db := GetDB()
	row := db.QueryRow(`SELECT status FROM jobs WHERE id = ?`, id)
	var job Job

	err := row.Scan(&job.Status)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Job not found error
		}
		return nil, err // Other errors
	}

	return &job.Status, nil
}

func GetJob(id string, withResult bool) (*Job, error) {
	db := GetDB()
	var row *sql.Row
	var job Job
	var createdAt string
	var fulfilledAt sql.NullString

	if withResult { // If the user wants the result
		row = db.QueryRow(
			`SELECT id, created_at, fulfilled_at, status, job_type, prompt, result, model FROM jobs WHERE id = ?`,
			id,
		)

		if err := row.Scan(&job.ID, &createdAt, &fulfilledAt, &job.Status, &job.JobType, &job.Prompt, &job.Result, &job.Model); err != nil {
			return nil, err
		}
	} else { // If the user doesn't want the result
		row = db.QueryRow(
			`SELECT id, created_at, fulfilled_at, status, job_type, prompt, model FROM jobs WHERE id = ?`,
			id,
		)

		if err := row.Scan(&job.ID, &createdAt, &fulfilledAt, &job.Status, &job.JobType, &job.Prompt, &job.Model); err != nil {
			return nil, err
		}
	}

	// Parse createdAt
	t, err := time.Parse(time.RFC3339Nano, createdAt)
	if err == nil {
		job.CreatedAt = t
	}
	// Parse fulfilledAt if present
	if fulfilledAt.Valid {
		ft, err := time.Parse(time.RFC3339Nano, fulfilledAt.String)
		if err == nil {
			job.FulfilledAt = &ft
		}
	}

	return &job, nil
}

func UpdateJobResult(id string, status JobStatus, result string) error {
	db := GetDB()
	now := time.Now().Format(time.RFC3339Nano)
	_, err := db.Exec(
		`UPDATE jobs SET status = ?, result = ?, fulfilled_at = ? WHERE id = ?`,
		status, result, now, id,
	)
	return err
}

func UpdateJobStatus(id string, status JobStatus) error {
	db := GetDB()
	_, err := db.Exec(
		`UPDATE jobs SET status = ? WHERE id = ?`,
		status, id,
	)
	return err
}

func ListJobs(limit int, withResult bool) ([]Job, error) {
	db := GetDB()
	var rows *sql.Rows
	var err error

	// Fetch jobs from the database
	if withResult {
		rows, err = db.Query(
			`SELECT id, created_at, fulfilled_at, status, job_type, prompt, result, model FROM jobs ORDER BY created_at DESC LIMIT ?`, limit)
	} else {
		rows, err = db.Query(
			`SELECT id, created_at, fulfilled_at, status, job_type, prompt, model FROM jobs ORDER BY created_at DESC LIMIT ?`, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var jobs []Job

	// Iterate through the rows and scan the data into Job structs
	for rows.Next() {
		var job Job
		var createdAt string
		var fulfilledAt sql.NullString
		if withResult {
			if err := rows.Scan(&job.ID, &createdAt, &fulfilledAt, &job.Status, &job.JobType, &job.Prompt, &job.Result, &job.Model); err != nil {
				continue
			}
		} else {
			if err := rows.Scan(&job.ID, &createdAt, &fulfilledAt, &job.Status, &job.JobType, &job.Prompt, &job.Model); err != nil {
				continue
			}
			job.Result = ""
		}
		job.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		if fulfilledAt.Valid {
			t, _ := time.Parse(time.RFC3339Nano, fulfilledAt.String)
			job.FulfilledAt = &t
		}
		jobs = append(jobs, job)
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
	rows, err := db.Query(`SELECT id, created_at, fulfilled_at, status, job_type, prompt, result, model, images_path FROM jobs WHERE status = ?`, JobPending)
	if err != nil {
		log.Printf("Error querying pending jobs: %v", err)
		return
	}
	defer rows.Close()

	// Iterate through the rows and process each job
	for rows.Next() {
		var job Job
		var createdAt string
		var fulfilledAt sql.NullString
		var imagesPathStr sql.NullString
		var resultStr sql.NullString // <-- Add this line

		// Scan all fields
		if err := rows.Scan(&job.ID, &createdAt, &fulfilledAt, &job.Status, &job.JobType, &job.Prompt, &resultStr, &job.Model, &imagesPathStr); err != nil {
			log.Printf("Error scanning job row: %v", err)
			continue
		}
		// Assign result if not null
		if resultStr.Valid {
			job.Result = resultStr.String
		} else {
			job.Result = ""
		}
		// Parse createdAt
		t, err := time.Parse(time.RFC3339Nano, createdAt)
		if err == nil {
			job.CreatedAt = t
		}
		// Parse fulfilledAt if present
		if fulfilledAt.Valid {
			ft, err := time.Parse(time.RFC3339Nano, fulfilledAt.String)
			if err == nil {
				job.FulfilledAt = &ft
			}
		}

		// Parse imagesPath if present
		if imagesPathStr.Valid && imagesPathStr.String != "" {
			_ = json.Unmarshal([]byte(imagesPathStr.String), &job.ImagesPath)
		}

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
			// var req MultiModalExtractionRequest
			url := GetOllamaURL()

			// Read the file bytes from the path
			if len(job.ImagesPath) > 0 {
				filePath := job.ImagesPath[0]
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
