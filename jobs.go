package main

import (
	"database/sql"
	"encoding/json"
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
	JobType     JobType    `json:"job_type"`
	Input       string     `json:"input"`
	Result      string     `json:"result,omitempty"`
}

func CreateJob(jobType JobType, input interface{}) (*Job, error) {
	id := uuid.New().String()
	now := time.Now()
	inputBytes, _ := json.Marshal(input)
	job := &Job{
		ID:        id,
		CreatedAt: now,
		Status:    JobPending,
		JobType:   jobType,
		Input:     string(inputBytes),
	}
	db := GetDB()
	_, err := db.Exec(
		`INSERT INTO jobs (id, created_at, status, job_type, input) VALUES (?, ?, ?, ?, ?)`,
		job.ID, job.CreatedAt, job.Status, job.JobType, job.Input,
	)
	if err != nil {
		return nil, err
	}
	return job, nil
}

func GetJob(id string) (*Job, error) {
	db := GetDB()
	row := db.QueryRow(`SELECT id, created_at, fulfilled_at, status, job_type, input, result FROM jobs WHERE id = ?`, id)
	var job Job
	var createdAt string
	var fulfilledAt sql.NullString
	err := row.Scan(&job.ID, &createdAt, &fulfilledAt, &job.Status, &job.JobType, &job.Input, &job.Result)
	if err != nil {
		return nil, err
	}
	job.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	if fulfilledAt.Valid {
		t, _ := time.Parse(time.RFC3339Nano, fulfilledAt.String)
		job.FulfilledAt = &t
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

func ListJobs(limit int) ([]Job, error) {
	db := GetDB()
	rows, err := db.Query(
		`SELECT id, created_at, fulfilled_at, status, job_type, input, result FROM jobs ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var jobs []Job
	for rows.Next() {
		var job Job
		var createdAt string
		var fulfilledAt sql.NullString
		if err := rows.Scan(&job.ID, &createdAt, &fulfilledAt, &job.Status, &job.JobType, &job.Input, &job.Result); err != nil {
			continue
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
		interval := 10 // default to 10 seconds
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
	db := GetDB()
	rows, err := db.Query(`SELECT id, job_type, input FROM jobs WHERE status = ?`, JobPending)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var jobType string
		var input string
		if err := rows.Scan(&id, &jobType, &input); err != nil {
			continue
		}
		UpdateJobStatus(id, JobRunning)
		var result string
		var status JobStatus
		switch JobType(jobType) {
		case JobTypeGenerate:
			var req GenerateRequest
			json.Unmarshal([]byte(input), &req)
			url := GetOllamaURL()
			resp, err := GenerateResponse(url, req)
			if err != nil {
				result = err.Error()
				status = JobFailed
			} else {
				b, _ := json.Marshal(resp)
				result = string(b)
				status = JobFulfilled
			}
		case JobTypeOCRExtract:
			var req struct {
				ModelName string `json:"model"`
				FileBytes []byte `json:"file_bytes"`
				Filename  string `json:"filename"`
			}
			json.Unmarshal([]byte(input), &req)
			url := GetOllamaURL()
			resp, err := ExtractTextFromImage(url, req.ModelName, req.FileBytes, req.Filename)
			if err != nil {
				result = err.Error()
				status = JobFailed
			} else {
				b, _ := json.Marshal(resp)
				result = string(b)
				status = JobFulfilled
			}
		default:
			result = "Unknown job type"
			status = JobFailed
		}
		UpdateJobResult(id, status, result)
	}
}

// Helper to get OLLAMA_URL from env
func GetOllamaURL() string {
	url := os.Getenv("OLLAMA_URL")
	return url
}

// Check if job result is still retrievable
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
