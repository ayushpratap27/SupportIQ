package services

import (
	"context"
	"fmt"
	"time"

	replyprovider "github.com/ayush/supportiq/internal/ai/reply/provider"
	replyprompt "github.com/ayush/supportiq/internal/ai/reply/prompt"
	"github.com/ayush/supportiq/internal/dto"
	"github.com/ayush/supportiq/internal/knowledge/retrieval"
	"github.com/ayush/supportiq/internal/models"
	"github.com/ayush/supportiq/internal/repositories"
	"github.com/ayush/supportiq/internal/utils"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ReplyService orchestrates AI reply generation, approval, editing, and rejection.
type ReplyService struct {
	replyProvider replyprovider.ReplyProvider
	retriever     retrieval.Retriever
	ticketRepo    *repositories.TicketRepository
	replyRepo     *repositories.ReplyRepository
	activityRepo  *repositories.ActivityRepository
	model         string
}

func NewReplyService(
	replyProvider replyprovider.ReplyProvider,
	retriever retrieval.Retriever,
	ticketRepo *repositories.TicketRepository,
	replyRepo *repositories.ReplyRepository,
	activityRepo *repositories.ActivityRepository,
	model string,
) *ReplyService {
	return &ReplyService{
		replyProvider: replyProvider,
		retriever:     retriever,
		ticketRepo:    ticketRepo,
		replyRepo:     replyRepo,
		activityRepo:  activityRepo,
		model:         model,
	}
}

// GenerateForTicket is called automatically after AI analysis completes.
func (s *ReplyService) GenerateForTicket(tenantID uuid.UUID, ticketID uuid.UUID, userID uint) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	log := utils.Logger.WithField("ticket_id", ticketID)
	log.Info("Reply: Starting automatic reply generation")
	if _, err := s.generate(ctx, tenantID, ticketID, userID); err != nil {
		log.WithError(err).Warn("Reply: Automatic generation failed")
	}
}

func (s *ReplyService) Generate(ctx context.Context, tenantID uuid.UUID, ticketID uuid.UUID, userID uint) (*models.AIReply, error) {
	return s.generate(ctx, tenantID, ticketID, userID)
}

func (s *ReplyService) Regenerate(ctx context.Context, tenantID uuid.UUID, ticketID uuid.UUID, userID uint) (*models.AIReply, error) {
	if latest, err := s.replyRepo.FindLatestByTicketID(tenantID, ticketID); err == nil {
		if latest.Status == models.AIReplyStatusGenerated {
			latest.Status = models.AIReplyStatusRegenerated
			_ = s.replyRepo.Update(latest)
		}
	}

	reply, err := s.generate(ctx, tenantID, ticketID, userID)
	if err != nil {
		return nil, err
	}

	_ = s.activityRepo.Create(&models.TicketActivity{
		TenantID:     tenantID,
		TicketID:     ticketID,
		UserID:       userID,
		ActivityType: models.ActivityReplyRegenerated,
		Description:  "AI reply regenerated",
	})

	return reply, nil
}

func (s *ReplyService) Approve(ctx context.Context, tenantID uuid.UUID, ticketID uuid.UUID, userID uint) (*models.AIReply, error) {
	reply, err := s.replyRepo.FindLatestByTicketID(tenantID, ticketID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("no reply draft found for this ticket")
		}
		return nil, err
	}

	if reply.Status != models.AIReplyStatusGenerated {
		return nil, fmt.Errorf("reply is in status %s and cannot be approved", reply.Status)
	}

	now := time.Now()
	reply.Status = models.AIReplyStatusApproved
	reply.ApprovedBy = &userID
	reply.ApprovedAt = &now

	if err := s.replyRepo.Update(reply); err != nil {
		return nil, err
	}

	_ = s.activityRepo.Create(&models.TicketActivity{
		TenantID:     tenantID,
		TicketID:     ticketID,
		UserID:       userID,
		ActivityType: models.ActivityReplyApproved,
		Description:  "AI reply approved by agent",
	})

	return reply, nil
}

func (s *ReplyService) Edit(ctx context.Context, tenantID uuid.UUID, ticketID uuid.UUID, userID uint, editedReply string) (*models.AIReply, error) {
	reply, err := s.replyRepo.FindLatestByTicketID(tenantID, ticketID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("no reply draft found for this ticket")
		}
		return nil, err
	}

	if reply.Status != models.AIReplyStatusGenerated {
		return nil, fmt.Errorf("only GENERATED replies can be edited; current status: %s", reply.Status)
	}

	reply.EditedReply = editedReply
	if err := s.replyRepo.Update(reply); err != nil {
		return nil, err
	}

	_ = s.activityRepo.Create(&models.TicketActivity{
		TenantID:     tenantID,
		TicketID:     ticketID,
		UserID:       userID,
		ActivityType: models.ActivityReplyEdited,
		Description:  "AI reply edited by agent",
	})

	return reply, nil
}

func (s *ReplyService) Reject(ctx context.Context, tenantID uuid.UUID, ticketID uuid.UUID, userID uint) (*models.AIReply, error) {
	reply, err := s.replyRepo.FindLatestByTicketID(tenantID, ticketID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("no reply draft found for this ticket")
		}
		return nil, err
	}

	if reply.Status != models.AIReplyStatusGenerated {
		return nil, fmt.Errorf("reply is in status %s and cannot be rejected", reply.Status)
	}

	reply.Status = models.AIReplyStatusRejected
	if err := s.replyRepo.Update(reply); err != nil {
		return nil, err
	}

	_ = s.activityRepo.Create(&models.TicketActivity{
		TenantID:     tenantID,
		TicketID:     ticketID,
		UserID:       userID,
		ActivityType: models.ActivityReplyRejected,
		Description:  "AI reply rejected by agent",
	})

	return reply, nil
}

func (s *ReplyService) GetLatest(tenantID uuid.UUID, ticketID uuid.UUID) (*models.AIReply, error) {
	return s.replyRepo.FindLatestByTicketID(tenantID, ticketID)
}

func (s *ReplyService) GetHistory(tenantID uuid.UUID, ticketID uuid.UUID) ([]dto.AIReplyResponse, error) {
	replies, err := s.replyRepo.FindAllByTicketID(tenantID, ticketID)
	if err != nil {
		return nil, err
	}
	result := make([]dto.AIReplyResponse, len(replies))
	for i, r := range replies {
		result[i] = toReplyResponse(&r)
	}
	return result, nil
}

// ─── Internal helpers ─────────────────────────────────────────────────────────

func (s *ReplyService) generate(ctx context.Context, tenantID uuid.UUID, ticketID uuid.UUID, userID uint) (*models.AIReply, error) {
	log := utils.Logger.WithField("ticket_id", ticketID)

	// 1. Load ticket (unscoped — called from worker context)
	ticket, err := s.ticketRepo.FindByIDUnscoped(ticketID)
	if err != nil {
		return nil, fmt.Errorf("ticket not found: %w", err)
	}

	// 2. Retrieve relevant KB documents (RAG)
	query := ticket.Subject
	if ticket.AICategory != "" {
		query += " " + ticket.AICategory
	}

	docs, err := s.retriever.Retrieve(ctx, tenantID, query, 5)
	if err != nil {
		log.WithError(err).Error("Reply: Knowledge base retrieval failed")
		return nil, fmt.Errorf("knowledge base unavailable: %w", err)
	}
	if len(docs) == 0 {
		log.Warn("Reply: No relevant knowledge base documents found")
		return nil, fmt.Errorf("knowledge base unavailable: no relevant documents found for this ticket — add knowledge base articles to enable AI reply generation")
	}

	log.WithField("docs_found", len(docs)).Info("Reply: Knowledge documents retrieved")

	// 3. Build provider request
	replyDocs := make([]replyprovider.RelevantDocument, len(docs))
	for i, d := range docs {
		replyDocs[i] = replyprovider.RelevantDocument{
			Title:    d.Title,
			Category: string(d.Category),
			Content:  d.Content,
		}
	}

	req := replyprovider.ReplyRequest{
		Subject:     ticket.Subject,
		Description: ticket.Description,
		Category:    ticket.AICategory,
		Priority:    ticket.AIPriority,
		Sentiment:   ticket.AISentiment,
		Documents:   replyDocs,
	}

	// 4. Call AI provider
	start := time.Now()
	result, err := s.replyProvider.GenerateReply(ctx, req)
	genTime := time.Since(start).Milliseconds()

	if err != nil {
		log.WithError(err).Error("Reply: AI provider failed")
		return nil, fmt.Errorf("AI reply generation failed: %w", err)
	}

	log.WithField("confidence", result.Confidence).
		WithField("gen_time_ms", genTime).
		Info("Reply: AI reply generated successfully")

	// 5. Persist reply
	reply := &models.AIReply{
		TenantID:       tenantID,
		TicketID:       ticketID,
		GeneratedReply: result.Reply,
		Confidence:     result.Confidence,
		Status:         models.AIReplyStatusGenerated,
		Model:          s.model,
		PromptVersion:  replyprompt.CurrentVersion,
		GenerationTime: genTime,
	}

	if err := s.replyRepo.Create(reply); err != nil {
		return nil, fmt.Errorf("failed to persist reply: %w", err)
	}

	// 6. Log activity
	_ = s.activityRepo.Create(&models.TicketActivity{
		TenantID:     tenantID,
		TicketID:     ticketID,
		UserID:       userID,
		ActivityType: models.ActivityReplyGenerated,
		Description:  "AI reply generated",
	})

	return reply, nil
}

// ToReplyResponse converts an AIReply model to the API response DTO.
func ToReplyResponse(r *models.AIReply) dto.AIReplyResponse {
	return toReplyResponse(r)
}

func toReplyResponse(r *models.AIReply) dto.AIReplyResponse {
	resp := dto.AIReplyResponse{
		ID:             r.ID,
		TicketID:       r.TicketID,
		GeneratedReply: r.GeneratedReply,
		EditedReply:    r.EditedReply,
		Confidence:     r.Confidence,
		Status:         string(r.Status),
		Model:          r.Model,
		PromptVersion:  r.PromptVersion,
		GenerationTime: r.GenerationTime,
		ApprovedBy:     r.ApprovedBy,
		ApprovedAt:     r.ApprovedAt,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
	}
	if r.Approver != nil {
		resp.Approver = &dto.ApproverResponse{
			ID:   r.Approver.ID,
			Name: r.Approver.Name,
		}
	}
	return resp
}
