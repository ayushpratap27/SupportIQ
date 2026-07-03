package repositories

import (
	"github.com/ayush/supportiq/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CommentRepository handles all database access for ticket comments.
type CommentRepository struct {
	db *gorm.DB
}

func NewCommentRepository(db *gorm.DB) *CommentRepository {
	return &CommentRepository{db: db}
}

func (r *CommentRepository) Create(comment *models.TicketComment) error {
	return r.db.Create(comment).Error
}

func (r *CommentRepository) FindByID(id uint) (*models.TicketComment, error) {
	var comment models.TicketComment
	if err := r.db.Preload("User").First(&comment, id).Error; err != nil {
		return nil, err
	}
	return &comment, nil
}

func (r *CommentRepository) ListByTicketID(ticketID uuid.UUID) ([]models.TicketComment, error) {
	var comments []models.TicketComment
	err := r.db.
		Preload("User").
		Where("ticket_id = ?", ticketID).
		Order("created_at ASC").
		Find(&comments).Error
	return comments, err
}
