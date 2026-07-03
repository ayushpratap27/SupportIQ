package repositories

import (
	"github.com/ayush/supportiq/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ActivityRepository handles immutable audit-log rows.
type ActivityRepository struct {
	db *gorm.DB
}

func NewActivityRepository(db *gorm.DB) *ActivityRepository {
	return &ActivityRepository{db: db}
}

func (r *ActivityRepository) Create(activity *models.TicketActivity) error {
	return r.db.Create(activity).Error
}

// ListByTicketID returns activities for a ticket scoped to a tenant.
func (r *ActivityRepository) ListByTicketID(tenantID uuid.UUID, ticketID uuid.UUID) ([]models.TicketActivity, error) {
	var activities []models.TicketActivity
	err := r.db.
		Preload("User").
		Where("tenant_id = ? AND ticket_id = ?", tenantID, ticketID).
		Order("created_at ASC").
		Find(&activities).Error
	return activities, err
}

// ListRecent returns the most recent N activities across all tickets for a tenant.
func (r *ActivityRepository) ListRecent(tenantID uuid.UUID, limit int) ([]models.TicketActivity, error) {
	var activities []models.TicketActivity
	err := r.db.
		Preload("User").
		Where("tenant_id = ?", tenantID).
		Order("created_at DESC").
		Limit(limit).
		Find(&activities).Error
	return activities, err
}

// FindActivitiesSince returns activities with ID > minID for integration worker.
func (r *ActivityRepository) FindActivitiesSince(tenantID uuid.UUID, minID uint, limit int) ([]models.TicketActivity, error) {
	var activities []models.TicketActivity
	err := r.db.Where("tenant_id = ? AND id > ?", tenantID, minID).
		Order("id ASC").
		Limit(limit).
		Find(&activities).Error
	return activities, err
}
