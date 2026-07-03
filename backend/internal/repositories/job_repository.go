package repositories

import (
	"time"

	"github.com/ayush/supportiq/internal/models"
	"gorm.io/gorm"
)

// JobRepository handles all database access for the background_jobs table.
type JobRepository struct {
	db *gorm.DB
}

func NewJobRepository(db *gorm.DB) *JobRepository {
	return &JobRepository{db: db}
}

// Create inserts a new background job record.
func (r *JobRepository) Create(job *models.BackgroundJob) error {
	return r.db.Create(job).Error
}

// FindByID loads a single job by primary key.
func (r *JobRepository) FindByID(id uint) (*models.BackgroundJob, error) {
	var job models.BackgroundJob
	if err := r.db.First(&job, id).Error; err != nil {
		return nil, err
	}
	return &job, nil
}

// MarkProcessing sets status to PROCESSING and records start time.
func (r *JobRepository) MarkProcessing(id uint) error {
	now := time.Now()
	return r.db.Model(&models.BackgroundJob{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     models.JobStatusProcessing,
		"started_at": now,
	}).Error
}

// MarkCompleted sets status to COMPLETED and records completion time.
func (r *JobRepository) MarkCompleted(id uint) error {
	now := time.Now()
	return r.db.Model(&models.BackgroundJob{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":       models.JobStatusCompleted,
		"completed_at": now,
	}).Error
}

// MarkFailed sets status to FAILED and records the error message.
func (r *JobRepository) MarkFailed(id uint, errMsg string) error {
	now := time.Now()
	return r.db.Model(&models.BackgroundJob{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":        models.JobStatusFailed,
		"error_message": errMsg,
		"completed_at":  now,
	}).Error
}

// MarkRetrying updates retry count and status.
func (r *JobRepository) MarkRetrying(id uint, retryCount int, errMsg string) error {
	return r.db.Model(&models.BackgroundJob{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":        models.JobStatusRetrying,
		"retry_count":   retryCount,
		"error_message": errMsg,
	}).Error
}

// MarkDead moves a job to the dead letter state.
func (r *JobRepository) MarkDead(id uint, errMsg string) error {
	now := time.Now()
	return r.db.Model(&models.BackgroundJob{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":        models.JobStatusDead,
		"error_message": errMsg,
		"completed_at":  now,
	}).Error
}

// List returns a paginated, optionally filtered list of jobs.
func (r *JobRepository) List(status, jobType string, page, limit int) ([]models.BackgroundJob, int64, error) {
	q := r.db.Model(&models.BackgroundJob{})

	if status != "" {
		q = q.Where("status = ?", status)
	}
	if jobType != "" {
		q = q.Where("job_type = ?", jobType)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	var jobs []models.BackgroundJob
	err := q.Order("created_at DESC").Offset(offset).Limit(limit).Find(&jobs).Error
	return jobs, total, err
}

// Statistics returns counts grouped by status.
func (r *JobRepository) Statistics() (map[string]int64, error) {
	type row struct {
		Status string
		Count  int64
	}

	var rows []row
	err := r.db.Model(&models.BackgroundJob{}).
		Select("status, count(*) as count").
		Group("status").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	result := make(map[string]int64)
	for _, row := range rows {
		result[row.Status] = row.Count
	}
	return result, nil
}
