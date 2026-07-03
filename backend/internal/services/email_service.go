package services

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ayush/supportiq/internal/dto"
	emailattachments "github.com/ayush/supportiq/internal/email/attachments"
	emailproviders "github.com/ayush/supportiq/internal/email/providers"
	"github.com/ayush/supportiq/internal/email/threading"
	"github.com/ayush/supportiq/internal/models"
	"github.com/ayush/supportiq/internal/repositories"
	"github.com/ayush/supportiq/internal/utils"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// EmailService handles inbound processing, outbound queuing, and ticket sync.
type EmailService struct {
	accountRepo  *repositories.EmailAccountRepository
	messageRepo  *repositories.EmailMessageRepository
	ticketRepo   *repositories.TicketRepository
	activityRepo *repositories.ActivityRepository
	accountSvc   *EmailAccountService
	detector     *threading.Detector
	storage      emailattachments.Storage
	jobSvc       *JobService // optional — for queuing AI analysis
	aiSvc        *AIService  // goroutine fallback
	db           *gorm.DB
}

// NewEmailService creates the email service with all dependencies.
func NewEmailService(
	accountRepo *repositories.EmailAccountRepository,
	messageRepo *repositories.EmailMessageRepository,
	ticketRepo *repositories.TicketRepository,
	activityRepo *repositories.ActivityRepository,
	accountSvc *EmailAccountService,
	detector *threading.Detector,
	storage emailattachments.Storage,
	aiSvc *AIService,
	db *gorm.DB,
) *EmailService {
	return &EmailService{
		accountRepo:  accountRepo,
		messageRepo:  messageRepo,
		ticketRepo:   ticketRepo,
		activityRepo: activityRepo,
		accountSvc:   accountSvc,
		detector:     detector,
		storage:      storage,
		aiSvc:        aiSvc,
		db:           db,
	}
}

// SetJobService injects the job service for Redis-backed AI analysis.
func (s *EmailService) SetJobService(js *JobService) { s.jobSvc = js }

// ── Inbound ───────────────────────────────────────────────────────────────────

// ProcessInbound processes one parsed inbound email: detects/creates a ticket,
// stores the message record, persists attachments, and logs activity.
func (s *EmailService) ProcessInbound(ctx context.Context, account *models.EmailAccount, parsed *emailproviders.ParsedEmail) error {
	// Deduplicate — skip if this Message-ID is already recorded
	if parsed.MessageID != "" {
		if _, err := s.messageRepo.FindTicketByMessageID(ctx, parsed.MessageID); err == nil {
			utils.Logger.WithField("message_id", parsed.MessageID).Debug("Email: duplicate, skipping")
			return nil
		}
	}

	// Thread detection — resolves to an existing ticket UUID or uuid.Nil
	ticketID, err := s.detector.Detect(ctx, parsed)
	if err != nil {
		return fmt.Errorf("email: thread detect: %w", err)
	}

	if ticketID == uuid.Nil {
		// New conversation — create ticket
		ticket, err := s.createTicketFromEmail(account, parsed)
		if err != nil {
			return fmt.Errorf("email: create ticket: %w", err)
		}
		ticketID = ticket.ID
		utils.Logger.WithField("ticket_id", ticketID).Info("Email: new ticket created")
	} else {
		utils.Logger.WithField("ticket_id", ticketID).Info("Email: appended to existing ticket")
	}

	// Persist email message record
	msg := &models.EmailMessage{
		TicketID:   ticketID,
		AccountID:  account.ID,
		MessageID:  parsed.MessageID,
		ThreadID:   parsed.ThreadID,
		InReplyTo:  parsed.InReplyTo,
		References: parsed.References,
		Direction:  models.EmailDirectionInbound,
		Sender:     parsed.From,
		Recipient:  parsed.To,
		Subject:    parsed.Subject,
		Body:       parsed.TextBody,
		HTMLBody:   parsed.HTMLBody,
		Status:     models.EmailStatusReceived,
		RawHeaders: parsed.RawHeaders,
	}

	now := parsed.Date
	if now.IsZero() {
		now = time.Now().UTC()
	}
	msg.ReceivedAt = &now

	// Save attachments and populate metadata
	for _, att := range parsed.Attachments {
		path, err := s.storage.Save(ticketID.String(), att.Filename, att.Data)
		if err != nil {
			utils.Logger.WithError(err).Warn("Email: failed to save attachment")
			continue
		}
		msg.Attachments = append(msg.Attachments, models.EmailAttachment{
			Filename:    att.Filename,
			ContentType: att.ContentType,
			Size:        att.Size,
			StoragePath: path,
		})
	}
	msg.AttachmentsCount = len(msg.Attachments)

	if err := s.messageRepo.Create(msg); err != nil {
		return fmt.Errorf("email: save message: %w", err)
	}

	// Activity log
	s.logActivity(ticketID, 0, models.ActivityEmailReceived,
		fmt.Sprintf("Email received from %s", parsed.FromAddress))

	if msg.AttachmentsCount > 0 {
		s.logActivity(ticketID, 0, models.ActivityAttachmentAdded,
			fmt.Sprintf("%d attachment(s) received", msg.AttachmentsCount))
	}

	// Trigger AI analysis (for newly created tickets only)
	return nil
}

// TriggerAIForTicket enqueues or runs AI analysis for a ticket.
// Should be called after ProcessInbound creates a new ticket.
func (s *EmailService) TriggerAIForTicket(ticketID uuid.UUID) {
	if s.jobSvc != nil && s.jobSvc.IsQueueAvailable() {
		_ = s.jobSvc.EnqueueAIAnalysis(ticketID, 0)
	} else {
		s.aiSvc.AnalyzeTicket(ticketID)
	}
}

// ── Outbound ──────────────────────────────────────────────────────────────────

// QueueReplyForTicket creates an OUTBOUND email message with status QUEUED.
// Called after an agent approves an AI reply.
func (s *EmailService) QueueReplyForTicket(ctx context.Context, ticketID uuid.UUID, replyText string, userID uint) error {
	// Find the active email account to send from
	accounts, err := s.accountRepo.FindActive()
	if err != nil || len(accounts) == 0 {
		return nil // no email account configured — silently skip
	}
	account := &accounts[0]
	if account.SMTPHost == "" {
		return nil
	}

	// Load ticket for customer details
	ticket, err := s.ticketRepo.FindByID(ticketID)
	if err != nil {
		return fmt.Errorf("email: load ticket: %w", err)
	}

	// Find the latest inbound message for threading headers
	var inReplyTo, references, subject string
	if prev, err := s.messageRepo.FindLatestInboundByTicket(ticketID); err == nil {
		inReplyTo = prev.MessageID
		references = buildReferences(prev.References, prev.MessageID)
		subject = "Re: " + strings.TrimPrefix(strings.TrimPrefix(prev.Subject, "Re: "), "RE: ")
	} else {
		subject = "Re: " + ticket.Subject
	}

	// Format the reply body
	body := formatReplyBody(replyText, ticket)

	msg := &models.EmailMessage{
		TicketID:   ticketID,
		AccountID:  account.ID,
		MessageID:  generateEmailMessageID(account.EmailAddress),
		InReplyTo:  inReplyTo,
		References: references,
		Direction:  models.EmailDirectionOutbound,
		Sender:     account.EmailAddress,
		Recipient:  ticket.CustomerEmail,
		Subject:    subject,
		Body:       body,
		Status:     models.EmailStatusQueued,
	}

	if err := s.messageRepo.Create(msg); err != nil {
		return fmt.Errorf("email: queue outbound: %w", err)
	}

	s.logActivity(ticketID, userID, models.ActivityEmailQueued,
		fmt.Sprintf("Outbound email queued to %s", ticket.CustomerEmail))

	return nil
}

// SendEmail sends a manually composed email immediately (admin action).
func (s *EmailService) SendEmail(ctx context.Context, ticketID uuid.UUID, req *dto.SendEmailRequest, userID uint) error {
	accounts, err := s.accountRepo.FindActive()
	if err != nil || len(accounts) == 0 {
		return fmt.Errorf("no active email account configured")
	}
	account := &accounts[0]
	if account.SMTPHost == "" {
		return fmt.Errorf("SMTP not configured for active account")
	}

	sender, err := s.accountSvc.BuildSender(account)
	if err != nil {
		return err
	}

	msgID := generateEmailMessageID(account.EmailAddress)
	outMsg := emailproviders.OutboundMessage{
		From:      fmt.Sprintf("%s <%s>", account.DisplayName, account.EmailAddress),
		To:        req.To,
		Subject:   req.Subject,
		TextBody:  req.Body,
		MessageID: msgID,
	}

	now := time.Now().UTC()
	status := models.EmailStatusSent
	errMsg := ""

	if sendErr := sender.Send(ctx, outMsg); sendErr != nil {
		utils.Logger.WithError(sendErr).Error("Email: SMTP send failed")
		status = models.EmailStatusFailed
		errMsg = sendErr.Error()
	}

	dbMsg := &models.EmailMessage{
		TicketID:  ticketID,
		AccountID: account.ID,
		MessageID: msgID,
		Direction: models.EmailDirectionOutbound,
		Sender:    account.EmailAddress,
		Recipient: req.To,
		Subject:   req.Subject,
		Body:      req.Body,
		Status:    status,
		SentAt:    &now,
	}
	if errMsg != "" {
		dbMsg.ErrorMessage = errMsg
	}

	if err := s.messageRepo.Create(dbMsg); err != nil {
		utils.Logger.WithError(err).Warn("Email: failed to persist sent message")
	}

	if status == models.EmailStatusSent {
		s.logActivity(ticketID, userID, models.ActivityEmailSent,
			fmt.Sprintf("Email sent to %s", req.To))
	} else {
		s.logActivity(ticketID, userID, models.ActivityEmailFailed,
			fmt.Sprintf("Email send failed: %s", errMsg))
	}

	if status == models.EmailStatusFailed {
		return fmt.Errorf("email send failed: %s", errMsg)
	}
	return nil
}

// ProcessQueuedOutbound fetches queued outbound messages and delivers them.
func (s *EmailService) ProcessQueuedOutbound(ctx context.Context) {
	msgs, err := s.messageRepo.FindQueued(50)
	if err != nil || len(msgs) == 0 {
		return
	}

	accounts, err := s.accountRepo.FindActive()
	if err != nil || len(accounts) == 0 {
		return
	}

	// Build a sender map indexed by account ID
	senders := make(map[uint]emailproviders.Sender)
	for i := range accounts {
		if accounts[i].SMTPHost == "" {
			continue
		}
		s2, err := s.accountSvc.BuildSender(&accounts[i])
		if err == nil {
			senders[accounts[i].ID] = s2
		}
	}

	for _, msg := range msgs {
		sender, ok := senders[msg.AccountID]
		if !ok {
			// fallback to first available sender
			for _, s2 := range senders {
				sender = s2
				break
			}
		}
		if sender == nil {
			continue
		}

		outMsg := emailproviders.OutboundMessage{
			To:         msg.Recipient,
			Subject:    msg.Subject,
			TextBody:   msg.Body,
			MessageID:  msg.MessageID,
			InReplyTo:  msg.InReplyTo,
			References: msg.References,
		}

		if sendErr := sender.Send(ctx, outMsg); sendErr != nil {
			utils.Logger.WithError(sendErr).WithField("msg_id", msg.ID).Warn("Email: outbound send failed")
			_ = s.messageRepo.IncrementRetry(msg.ID)
			_ = s.messageRepo.UpdateStatus(msg.ID, models.EmailStatusFailed, sendErr.Error())
			s.logActivity(msg.TicketID, 0, models.ActivityEmailFailed, sendErr.Error())
			continue
		}

		_ = s.messageRepo.UpdateStatus(msg.ID, models.EmailStatusSent, "")
		s.logActivity(msg.TicketID, 0, models.ActivityEmailSent,
			fmt.Sprintf("Email sent to %s", msg.Recipient))
	}
}

// RetryFailedOutbound retries failed messages that have not exceeded the max attempt count.
func (s *EmailService) RetryFailedOutbound(ctx context.Context, maxRetries int) {
	msgs, err := s.messageRepo.FindFailedRetryable(maxRetries)
	if err != nil || len(msgs) == 0 {
		return
	}
	for i := range msgs {
		msgs[i].Status = models.EmailStatusQueued
	}
	// Re-queue them — ProcessQueuedOutbound will pick them up on next tick
	for _, msg := range msgs {
		_ = s.messageRepo.UpdateStatus(msg.ID, models.EmailStatusQueued, "")
	}
}

// ── Ticket email list ─────────────────────────────────────────────────────────

// GetTicketEmails returns all email messages linked to a ticket.
func (s *EmailService) GetTicketEmails(ticketID uuid.UUID) ([]dto.EmailMessageResponse, int, error) {
	msgs, err := s.messageRepo.FindByTicketID(ticketID)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to fetch emails")
	}
	resp := make([]dto.EmailMessageResponse, len(msgs))
	for i := range msgs {
		resp[i] = dto.ToEmailMessageResponse(&msgs[i])
	}
	return resp, http.StatusOK, nil
}

// ── Monitor ───────────────────────────────────────────────────────────────────

// GetMonitorStats returns email health metrics for the admin dashboard.
func (s *EmailService) GetMonitorStats() (*dto.EmailMonitorStats, error) {
	total, _ := s.accountRepo.Count()
	active, _ := s.accountRepo.CountActive()
	queued, _ := s.messageRepo.CountByStatus(models.EmailStatusQueued)
	failed, _ := s.messageRepo.CountByStatus(models.EmailStatusFailed)
	sentToday, _ := s.messageRepo.CountSentToday()
	receivedToday, _ := s.messageRepo.CountReceivedToday()

	accounts, _ := s.accountRepo.FindAll()
	var lastSync *time.Time
	var accountResp []dto.EmailAccountResponse
	for i := range accounts {
		accountResp = append(accountResp, dto.ToEmailAccountResponse(&accounts[i]))
		if accounts[i].LastSyncAt != nil {
			if lastSync == nil || accounts[i].LastSyncAt.After(*lastSync) {
				lastSync = accounts[i].LastSyncAt
			}
		}
	}

	return &dto.EmailMonitorStats{
		TotalAccounts:  total,
		ActiveAccounts: active,
		QueuedCount:    queued,
		FailedCount:    failed,
		SentToday:      sentToday,
		ReceivedToday:  receivedToday,
		LastSyncAt:     lastSync,
		Accounts:       accountResp,
	}, nil
}

// ── Internal helpers ──────────────────────────────────────────────────────────

func (s *EmailService) createTicketFromEmail(account *models.EmailAccount, p *emailproviders.ParsedEmail) (*models.Ticket, error) {
	systemUserID := s.getSystemUserID()

	description := p.TextBody
	if description == "" {
		description = "(No text body)"
	}
	customerName := p.FromName
	if customerName == "" {
		customerName = p.FromAddress
	}
	if customerName == "" {
		customerName = p.From
	}

	tx := s.db.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	ticketRepo := repositories.NewTicketRepository(s.db)
	ticketNumber, err := ticketRepo.NextTicketNumber(tx)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	ticket := &models.Ticket{
		TicketNumber:  ticketNumber,
		Subject:       p.Subject,
		Description:   description,
		Status:        models.TicketStatusOpen,
		Priority:      models.TicketPriorityMedium,
		Category:      models.TicketCategoryGeneral,
		Source:        models.TicketSourceEmail,
		CustomerName:  customerName,
		CustomerEmail: p.FromAddress,
		CreatedBy:     systemUserID,
		AIProcessingStatus: models.AIStatusPending,
	}

	if err := ticketRepo.Create(tx, ticket); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return ticket, nil
}

func (s *EmailService) getSystemUserID() uint {
	var user models.User
	if err := s.db.Where("role = ?", models.RoleAdmin).Order("id asc").First(&user).Error; err != nil {
		return 1
	}
	return user.ID
}

func (s *EmailService) logActivity(ticketID uuid.UUID, userID uint, actType, desc string) {
	if userID == 0 {
		userID = s.getSystemUserID()
	}
	_ = s.activityRepo.Create(&models.TicketActivity{
		TicketID:     ticketID,
		UserID:       userID,
		ActivityType: actType,
		Description:  desc,
	})
}

// generateEmailMessageID creates a unique RFC 2822 Message-ID.
func generateEmailMessageID(fromAddr string) string {
	domain := "supportiq.local"
	if parts := strings.SplitN(fromAddr, "@", 2); len(parts) == 2 {
		domain = parts[1]
	}
	return fmt.Sprintf("%s@%s", uuid.New().String(), domain)
}

// formatReplyBody wraps the AI reply in a professional support email format.
func formatReplyBody(reply string, ticket *models.Ticket) string {
	return fmt.Sprintf("Dear %s,\n\nThank you for contacting us.\n\n%s\n\nBest regards,\nSupport Team\n\n---\nRef: %s",
		ticket.CustomerName, reply, ticket.TicketNumber)
}

// buildReferences appends a new Message-ID to an existing References chain.
func buildReferences(existing, newID string) string {
	existing = strings.TrimSpace(existing)
	newID = strings.Trim(newID, "<>")
	if newID == "" {
		return existing
	}
	formatted := "<" + newID + ">"
	if existing == "" {
		return formatted
	}
	return existing + " " + formatted
}
