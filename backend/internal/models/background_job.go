package models

import "time"

// JobStatus represents the lifecycle state of a background job.
type JobStatus string

// JobType represents the category of work a background job performs.
type JobType string

const (
	JobStatusQueued     JobStatus = "QUEUED"
	JobStatusProcessing JobStatus = "PROCESSING"
	JobStatusCompleted  JobStatus = "COMPLETED"
	JobStatusFailed     JobStatus = "FAILED"
	JobStatusRetrying   JobStatus = "RETRYING"
	JobStatusDead       JobStatus = "DEAD"
)

const (
	JobTypeAIAnalysis      JobType = "AI_ANALYSIS"
	JobTypeGenerateReply   JobType = "GENERATE_REPLY"
	JobTypeRegenerateReply JobType = "REGENERATE_REPLY"
	JobTypeRetryAI         JobType = "RETRY_AI"
	JobTypeRetryReply      JobType = "RETRY_REPLY"
)

// BackgroundJob tracks every async job in PostgreSQL for monitoring and auditing.
type BackgroundJob struct {
	ID           uint       `gorm:"primarykey;autoIncrement"              json:"id"`
	JobType      JobType    `gorm:"type:varchar(50);not null;index"       json:"job_type"`
	ReferenceID  string     `gorm:"type:varchar(255);not null;index"      json:"reference_id"` // ticket UUID
	Status       JobStatus  `gorm:"type:varchar(20);not null;default:'QUEUED';index" json:"status"`
	RetryCount   int        `gorm:"not null;default:0"                    json:"retry_count"`
	Payload      string     `gorm:"type:text"                             json:"payload,omitempty"`
	ErrorMessage string     `gorm:"type:text"                             json:"error_message,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
}
