package repositories

import (
	"github.com/ayush/supportiq/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// NoteRepository handles all database access for ticket notes.
type NoteRepository struct {
	db *gorm.DB
}

func NewNoteRepository(db *gorm.DB) *NoteRepository {
	return &NoteRepository{db: db}
}

func (r *NoteRepository) Create(note *models.TicketNote) error {
	return r.db.Create(note).Error
}

func (r *NoteRepository) FindByID(id uint) (*models.TicketNote, error) {
	var note models.TicketNote
	if err := r.db.Preload("User").First(&note, id).Error; err != nil {
		return nil, err
	}
	return &note, nil
}

func (r *NoteRepository) ListByTicketID(ticketID uuid.UUID) ([]models.TicketNote, error) {
	var notes []models.TicketNote
	err := r.db.
		Preload("User").
		Where("ticket_id = ?", ticketID).
		Order("created_at DESC").
		Find(&notes).Error
	return notes, err
}
