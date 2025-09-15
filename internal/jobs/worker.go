package jobs

import (
	"encoding/json"
	"log"
	"os"
	"strconv"
	"time"

	"zllm/internal/database"
	"zllm/internal/models"
	"zllm/internal/ollama"
)

// StartJobWorker starts the background job worker
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
	db := database.GetDB()
	var jobs []models.Job
	err := db.Where("status = ?", models.JobPending).Find(&jobs).Error
	if err != nil {
		log.Printf("Error querying pending jobs: %v", err)
		return
	}

	// Iterate through the jobs and process each one
	for _, job := range jobs {
		log.Printf("Processing job: ID=%s, Type=%s, Model=%s", job.ID, job.JobType, job.Model)

		// Update job status to running
		UpdateJobStatus(job.ID, models.JobRunning)

		var result string
		var status models.JobStatus
		// Create Ollama client
		client := ollama.NewClient(GetOllamaURL())

		switch job.JobType {
		case models.JobTypeGenerate: // Handle generation jobs
			req := ollama.GenerationRequest{
				Prompt: job.Prompt,
				Model:  job.Model,
			}

			resp, err := client.GenerateResponse(req)
			if err != nil {
				result = err.Error()
				status = models.JobFailed
				log.Printf("Job %s failed (generation): %v", job.ID, err)
			} else {
				if jsonBytes, err := json.Marshal(resp); err == nil {
					result = string(jsonBytes)
					status = models.JobFulfilled
					log.Printf("Job %s fulfilled (generation)", job.ID)
				} else {
					result = err.Error()
					status = models.JobFailed
					log.Printf("Job %s failed to marshal response: %v", job.ID, err)
				}
			}
		case models.JobTypeOCRExtract: // Handle MultiModal Extraction jobs
			imagesPath := job.GetImagesPathSlice()

			// Read the file bytes from the path
			if len(imagesPath) > 0 {
				filePath := imagesPath[0]
				fileBytes, err := os.ReadFile(filePath)
				if err != nil {
					result = err.Error()
					status = models.JobFailed
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
					resp, err_ := client.MultiModalTextExtractionFromImage(job.Model, fileBytes, filename)
					if err_ != nil {
						result = err_.Error()
						status = models.JobFailed
						log.Printf("Job %s failed (OCR extraction): %v", job.ID, err_)
					} else {
						if jsonBytes, err := json.Marshal(resp); err == nil {
							result = string(jsonBytes)
							status = models.JobFulfilled
							log.Printf("Job %s fulfilled (OCR extraction)", job.ID)
						} else {
							result = err.Error()
							status = models.JobFailed
							log.Printf("Job %s failed to marshal OCR response: %v", job.ID, err)
						}
					}
					// Clean up the temporary file
					os.Remove(filePath)
				}
			} else {
				result = "No image path found"
				status = models.JobFailed
				log.Printf("Job %s failed: no image path found", job.ID)
			}
		default: // Handle unknown job types
			result = ""
			status = models.JobFailed
			log.Printf("Job %s failed: unknown job type %s", job.ID, job.JobType)
		}

		// Update job result and status in the database
		UpdateJobResult(job.ID, status, result)
		log.Printf("Job %s updated with status %s", job.ID, status)
	}
}

// GetOllamaURL helper to get OLLAMA_URL from env
func GetOllamaURL() string {
	url := os.Getenv("OLLAMA_URL")
	return url
}