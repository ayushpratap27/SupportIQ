package repositories

import (
	"fmt"
	"strings"

	"github.com/ayush/supportiq/internal/dto"
	"github.com/ayush/supportiq/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TicketRepository encapsulates all database access for tickets.
type TicketRepository struct {
	db *gorm.DB
}

func NewTicketRepository(db *gorm.DB) *TicketRepository {
	return &TicketRepository{db: db}
}

// DB exposes the raw connection so the service layer can start transactions.
func (r *TicketRepository) DB() *gorm.DB {
	return r.db
}

// scoped returns a DB session scoped to the given tenant.
func (r *TicketRepository) scoped(tenantID uuid.UUID) *gorm.DB {
	return r.db.Where("tenant_id = ?", tenantID)
}

// NextTicketNumber generates the next sequential ticket number for a tenant
// using a row-level lock to ensure uniqueness under concurrent writes.
func (r *TicketRepository) NextTicketNumber(tenantID uuid.UUID, tx *gorm.DB) (string, error) {
	var counter models.TicketCounter
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		FirstOrCreate(&counter, models.TicketCounter{TenantID: tenantID}).Error
	if err != nil {
		return "", fmt.Errorf("failed to lock ticket counter: %w", err)
	}
	counter.LastValue++
	if err := tx.Save(&counter).Error; err != nil {
		return "", fmt.Errorf("failed to update ticket counter: %w", err)
	}
	return models.FormatTicketNumber(counter.LastValue), nil
}

// Create inserts a new ticket using the provided transaction.
func (r *TicketRepository) Create(tx *gorm.DB, t *models.Ticket) error {
	return tx.Create(t).Error
}

// FindByID loads a single ticket scoped to the tenant.
func (r *TicketRepository) FindByID(tenantID uuid.UUID, id uuid.UUID) (*models.Ticket, error) {
	var t models.Ticket
	err := r.scoped(tenantID).
		Preload("Creator").
		Preload("Assignee").
		First(&t, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// FindByIDUnscoped loads a ticket without tenant check (used by background workers).
func (r *TicketRepository) FindByIDUnscoped(id uuid.UUID) (*models.Ticket, error) {
	var t models.Ticket
	err := r.db.Preload("Creator").Preload("Assignee").First(&t, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// Update persists all changes to an existing ticket record.
func (r *TicketRepository) Update(t *models.Ticket) error {
	return r.db.Save(t).Error
}

// SoftDelete marks a ticket as deleted without removing the row.
func (r *TicketRepository) SoftDelete(tenantID uuid.UUID, id uuid.UUID) error {
	return r.scoped(tenantID).Delete(&models.Ticket{}, "id = ?", id).Error
}

// UpdateAIFields persists only the AI analysis columns.
func (r *TicketRepository) UpdateAIFields(t *models.Ticket) error {
	return r.db.Model(t).
		Select("AICategory", "AIPriority", "AISentiment", "AITeam",
			"AIConfidence", "AISummary", "AITags", "AIProcessingStatus", "ProcessedAt").
		Updates(t).Error
}

// List returns a filtered, paginated slice of tickets for a tenant.
func (r *TicketRepository) List(tenantID uuid.UUID, q *dto.ListTicketsQuery) ([]models.Ticket, int64, error) {
	base := r.scoped(tenantID).Model(&models.Ticket{}).
		Preload("Creator").
		Preload("Assignee")

	if q.Search != "" {
		term := "%" + strings.ToLower(q.Search) + "%"
		base = base.Where(
			"LOWER(subject) LIKE ? OR LOWER(description) LIKE ? OR LOWER(ticket_number) LIKE ? OR LOWER(customer_name) LIKE ?",
			term, term, term, term,
		)
	}
	if q.Status != "" {
		base = base.Where("status = ?", q.Status)
	}
	if q.Priority != "" {
		base = base.Where("priority = ?", q.Priority)
	}
	if q.Category != "" {
		base = base.Where("category = ?", q.Category)
	}
	if q.UnassignedOnly {
		base = base.Where("assigned_to IS NULL")
	} else if q.AssignedTo != nil {
		base = base.Where("assigned_to = ?", *q.AssignedTo)
	}
	if q.CreatedBy != nil {
		base = base.Where("created_by = ?", *q.CreatedBy)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var tickets []models.Ticket
	offset := (q.Page - 1) * q.Limit
	if err := base.Order("created_at DESC").Offset(offset).Limit(q.Limit).Find(&tickets).Error; err != nil {
		return nil, 0, err
	}

	return tickets, total, nil
}
