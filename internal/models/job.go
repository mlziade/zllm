package models

import (
	"encoding/json"
	"time"
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