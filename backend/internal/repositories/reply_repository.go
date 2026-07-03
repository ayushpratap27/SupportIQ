package repositories

import (
	"github.com/ayush/supportiq/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ReplyRepository handles all database access for the ai_replies table.
type ReplyRepository struct {
	db *gorm.DB
}

func NewReplyRepository(db *gorm.DB) *ReplyRepository {
	return &ReplyRepository{db: db}
}

// Create inserts a new AI reply record.
func (r *ReplyRepository) Create(reply *models.AIReply) error {
	return r.db.Create(reply).Error
}

// Update persists changes to an existing reply record.
func (r *ReplyRepository) Update(reply *models.AIReply) error {
	return r.db.Save(reply).Error
}

// FindLatestByTicketID returns the most recently created reply for a ticket.
func (r *ReplyRepository) FindLatestByTicketID(ticketID uuid.UUID) (*models.AIReply, error) {
	var reply models.AIReply
	err := r.db.
		Preload("Approver").
		Where("ticket_id = ?", ticketID).
		Order("created_at DESC").
		First(&reply).Error
	if err != nil {
		return nil, err
	}
	return &reply, nil
}

// FindAllByTicketID returns the full generation history for a ticket, newest first.
func (r *ReplyRepository) FindAllByTicketID(ticketID uuid.UUID) ([]models.AIReply, error) {
	var replies []models.AIReply
	err := r.db.
		Preload("Approver").
		Where("ticket_id = ?", ticketID).
		Order("created_at DESC").
		Find(&replies).Error
	return replies, err
}
