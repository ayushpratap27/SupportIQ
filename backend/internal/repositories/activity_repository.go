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

// ListByTicketID returns activities for a ticket in chronological order.
func (r *ActivityRepository) ListByTicketID(ticketID uuid.UUID) ([]models.TicketActivity, error) {
	var activities []models.TicketActivity
	err := r.db.
		Preload("User").
		Where("ticket_id = ?", ticketID).
		Order("created_at ASC").
		Find(&activities).Error
	return activities, err
}

// ListRecent returns the most recent N activities across all tickets.
func (r *ActivityRepository) ListRecent(limit int) ([]models.TicketActivity, error) {
	var activities []models.TicketActivity
	err := r.db.
		Preload("User").
		Order("created_at DESC").
		Limit(limit).
		Find(&activities).Error
	return activities, err
}
