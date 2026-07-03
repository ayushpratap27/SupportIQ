package repositories

import (
	"time"

	"github.com/ayush/supportiq/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// IntegrationRepository handles all database access for integrations.
type IntegrationRepository struct {
	db *gorm.DB
}

func NewIntegrationRepository(db *gorm.DB) *IntegrationRepository {
	return &IntegrationRepository{db: db}
}

// ─── Integration CRUD ───────────────────────────────────────────────────────

func (r *IntegrationRepository) Create(integration *models.Integration) error {
	return r.db.Create(integration).Error
}

func (r *IntegrationRepository) FindByID(id uint) (*models.Integration, error) {
	var integration models.Integration
	err := r.db.Preload("Creator").First(&integration, id).Error
	if err != nil {
		return nil, err
	}
	return &integration, nil
}

func (r *IntegrationRepository) FindAll() ([]models.Integration, error) {
	var integrations []models.Integration
	err := r.db.Order("created_at DESC").Find(&integrations).Error
	return integrations, err
}

func (r *IntegrationRepository) FindEnabled() ([]models.Integration, error) {
	var integrations []models.Integration
	err := r.db.Where("enabled = true AND status != ?", models.IntegrationStatusError).
		Order("created_at DESC").
		Find(&integrations).Error
	return integrations, err
}

func (r *IntegrationRepository) FindByProvider(provider models.IntegrationProvider) ([]models.Integration, error) {
	var integrations []models.Integration
	err := r.db.Where("provider = ? AND enabled = true", provider).Find(&integrations).Error
	return integrations, err
}

func (r *IntegrationRepository) Update(integration *models.Integration) error {
	return r.db.Save(integration).Error
}

func (r *IntegrationRepository) Delete(id uint) error {
	return r.db.Delete(&models.Integration{}, id).Error
}

// ─── IntegrationEvent ───────────────────────────────────────────────────────

func (r *IntegrationRepository) CreateEvent(event *models.IntegrationEvent) error {
	return r.db.Create(event).Error
}

func (r *IntegrationRepository) FindPendingEvents(limit int) ([]models.IntegrationEvent, error) {
	var events []models.IntegrationEvent
	err := r.db.
		Where("status IN (?, ?)", models.IntEventPending, models.IntEventFailed).
		Where("retry_count < 5").
		Order("created_at ASC").
		Limit(limit).
		Find(&events).Error
	return events, err
}

func (r *IntegrationRepository) UpdateEventStatus(id uint, status models.IntegrationEventStatus, errMsg string) error {
	now := time.Now()
	updates := map[string]interface{}{
		"status":       status,
		"error_message": errMsg,
	}
	if status == models.IntEventProcessed {
		updates["processed_at"] = now
	}
	return r.db.Model(&models.IntegrationEvent{}).Where("id = ?", id).Updates(updates).Error
}

func (r *IntegrationRepository) IncrementEventRetry(id uint) error {
	return r.db.Model(&models.IntegrationEvent{}).Where("id = ?", id).
		UpdateColumn("retry_count", gorm.Expr("retry_count + 1")).Error
}

func (r *IntegrationRepository) MarkEventDead(id uint, errMsg string) error {
	return r.UpdateEventStatus(id, models.IntEventDead, errMsg)
}

// ─── TicketIntegration ──────────────────────────────────────────────────────

func (r *IntegrationRepository) CreateTicketIntegration(ti *models.TicketIntegration) error {
	return r.db.Create(ti).Error
}

func (r *IntegrationRepository) FindTicketIntegrations(ticketID uuid.UUID) ([]models.TicketIntegration, error) {
	var items []models.TicketIntegration
	err := r.db.Preload("Integration").
		Where("ticket_id = ?", ticketID).
		Find(&items).Error
	return items, err
}

func (r *IntegrationRepository) FindTicketIntegrationByProvider(ticketID uuid.UUID, integrationID uint) (*models.TicketIntegration, error) {
	var ti models.TicketIntegration
	err := r.db.Where("ticket_id = ? AND integration_id = ?", ticketID, integrationID).
		First(&ti).Error
	if err != nil {
		return nil, err
	}
	return &ti, nil
}

// FindSinceActivityID returns activities with ID > minID, ordered ascending.
func (r *IntegrationRepository) FindActivitiesSince(minID uint, limit int) ([]models.TicketActivity, error) {
	var activities []models.TicketActivity
	err := r.db.Where("id > ?", minID).
		Order("id ASC").
		Limit(limit).
		Find(&activities).Error
	return activities, err
}
