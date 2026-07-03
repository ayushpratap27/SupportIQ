package repositories

import (
	"context"
	"time"

	"github.com/ayush/supportiq/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// EmailMessageRepository encapsulates all database access for email_messages.
type EmailMessageRepository struct {
	db *gorm.DB
}

func NewEmailMessageRepository(db *gorm.DB) *EmailMessageRepository {
	return &EmailMessageRepository{db: db}
}

func (r *EmailMessageRepository) Create(m *models.EmailMessage) error {
	return r.db.Create(m).Error
}

func (r *EmailMessageRepository) FindByID(id uint) (*models.EmailMessage, error) {
	var m models.EmailMessage
	if err := r.db.First(&m, id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// FindByTicketID returns all emails for a ticket ordered oldest-first.
func (r *EmailMessageRepository) FindByTicketID(ticketID uuid.UUID) ([]models.EmailMessage, error) {
	var msgs []models.EmailMessage
	err := r.db.Where("ticket_id = ?", ticketID).
		Order("created_at asc").
		Find(&msgs).Error
	return msgs, err
}

// FindTicketByMessageID resolves an RFC 2822 Message-ID to a ticket UUID.
func (r *EmailMessageRepository) FindTicketByMessageID(_ context.Context, messageID string) (uuid.UUID, error) {
	var m models.EmailMessage
	if err := r.db.Select("ticket_id").
		Where("message_id = ?", messageID).
		First(&m).Error; err != nil {
		return uuid.Nil, err
	}
	return m.TicketID, nil
}

// FindTicketByThreadID resolves a thread-ID header value to a ticket UUID.
func (r *EmailMessageRepository) FindTicketByThreadID(_ context.Context, threadID string) (uuid.UUID, error) {
	var m models.EmailMessage
	if err := r.db.Select("ticket_id").
		Where("thread_id = ?", threadID).
		First(&m).Error; err != nil {
		return uuid.Nil, err
	}
	return m.TicketID, nil
}

// FindTicketBySubject tries to match an open ticket that received an email with
// the same normalised subject from the same sender address within 30 days.
func (r *EmailMessageRepository) FindTicketBySubject(_ context.Context, subject, fromAddress string) (uuid.UUID, error) {
	var m models.EmailMessage
	since := time.Now().AddDate(0, 0, -30)
	if err := r.db.Select("ticket_id").
		Where("subject = ? AND sender LIKE ? AND created_at >= ?",
			subject, "%"+fromAddress+"%", since).
		Order("created_at desc").
		First(&m).Error; err != nil {
		return uuid.Nil, err
	}
	return m.TicketID, nil
}

// FindLatestInboundByTicket returns the last INBOUND message for threading.
func (r *EmailMessageRepository) FindLatestInboundByTicket(ticketID uuid.UUID) (*models.EmailMessage, error) {
	var m models.EmailMessage
	if err := r.db.
		Where("ticket_id = ? AND direction = ?", ticketID, models.EmailDirectionInbound).
		Order("created_at desc").
		First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// FindQueued returns all outbound messages with status QUEUED ordered oldest-first.
func (r *EmailMessageRepository) FindQueued(limit int) ([]models.EmailMessage, error) {
	var msgs []models.EmailMessage
	err := r.db.Where("direction = ? AND status = ?",
		models.EmailDirectionOutbound, models.EmailStatusQueued).
		Order("created_at asc").
		Limit(limit).
		Find(&msgs).Error
	return msgs, err
}

// FindFailedRetryable returns FAILED outbound messages with retry_count < maxRetries.
func (r *EmailMessageRepository) FindFailedRetryable(maxRetries int) ([]models.EmailMessage, error) {
	var msgs []models.EmailMessage
	err := r.db.Where("direction = ? AND status = ? AND retry_count < ?",
		models.EmailDirectionOutbound, models.EmailStatusFailed, maxRetries).
		Order("created_at asc").
		Find(&msgs).Error
	return msgs, err
}

func (r *EmailMessageRepository) UpdateStatus(id uint, status models.EmailStatus, errMsg string) error {
	updates := map[string]interface{}{"status": status}
	if errMsg != "" {
		updates["error_message"] = errMsg
	}
	if status == models.EmailStatusSent {
		now := time.Now()
		updates["sent_at"] = now
	}
	return r.db.Model(&models.EmailMessage{}).Where("id = ?", id).Updates(updates).Error
}

func (r *EmailMessageRepository) IncrementRetry(id uint) error {
	return r.db.Model(&models.EmailMessage{}).Where("id = ?", id).
		UpdateColumn("retry_count", gorm.Expr("retry_count + 1")).Error
}

// ── Statistics ────────────────────────────────────────────────────────────────

func (r *EmailMessageRepository) CountByStatus(status models.EmailStatus) (int64, error) {
	var n int64
	return n, r.db.Model(&models.EmailMessage{}).Where("status = ?", status).Count(&n).Error
}

func (r *EmailMessageRepository) CountSentToday() (int64, error) {
	var n int64
	today := time.Now().Truncate(24 * time.Hour)
	return n, r.db.Model(&models.EmailMessage{}).
		Where("direction = ? AND status = ? AND sent_at >= ?",
			models.EmailDirectionOutbound, models.EmailStatusSent, today).
		Count(&n).Error
}

func (r *EmailMessageRepository) CountReceivedToday() (int64, error) {
	var n int64
	today := time.Now().Truncate(24 * time.Hour)
	return n, r.db.Model(&models.EmailMessage{}).
		Where("direction = ? AND created_at >= ?",
			models.EmailDirectionInbound, today).
		Count(&n).Error
}
