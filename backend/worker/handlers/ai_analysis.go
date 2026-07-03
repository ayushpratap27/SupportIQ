package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/ayush/supportiq/internal/ai/provider"
	"github.com/ayush/supportiq/internal/models"
	"github.com/ayush/supportiq/internal/queue"
	"github.com/ayush/supportiq/internal/repositories"
	"github.com/ayush/supportiq/internal/utils"
	"github.com/google/uuid"
)

// AIAnalysisHandler processes AI_ANALYSIS and RETRY_AI jobs.
type AIAnalysisHandler struct {
	ticketRepo   *repositories.TicketRepository
	activityRepo *repositories.ActivityRepository
	aiProvider   provider.Provider
}

func NewAIAnalysisHandler(
	ticketRepo *repositories.TicketRepository,
	activityRepo *repositories.ActivityRepository,
	aiProvider provider.Provider,
) *AIAnalysisHandler {
	return &AIAnalysisHandler{
		ticketRepo:   ticketRepo,
		activityRepo: activityRepo,
		aiProvider:   aiProvider,
	}
}

// Handle runs AI analysis for the given ticket job.
func (h *AIAnalysisHandler) Handle(ctx context.Context, job queue.Job) error {
	ticketID, err := uuid.Parse(job.TicketID)
	if err != nil {
		return fmt.Errorf("invalid ticket UUID %q: %w", job.TicketID, err)
	}

	log := utils.Logger.
		WithField("job_type", job.Type).
		WithField("ticket_id", ticketID)

	ticket, err := h.ticketRepo.FindByIDUnscoped(ticketID)
	if err != nil {
		return fmt.Errorf("ticket not found: %w", err)
	}
	tenantID := ticket.TenantID

	log.Info("AI Analysis: Starting")

	// Mark ticket as processing
	ticket.AIProcessingStatus = models.AIStatusProcessing
	_ = h.ticketRepo.UpdateAIFields(ticket)

	result, err := h.aiProvider.Analyze(ctx, provider.AnalysisRequest{
		Subject:      ticket.Subject,
		Description:  ticket.Description,
		CustomerName: ticket.CustomerName,
		Category:     string(ticket.Category),
		Priority:     string(ticket.Priority),
	})
	if err != nil {
		ticket.AIProcessingStatus = models.AIStatusFailed
		_ = h.ticketRepo.UpdateAIFields(ticket)
		return fmt.Errorf("AI analysis failed: %w", err)
	}

	// Persist results
	now := time.Now()
	ticket.AICategory = result.Category
	ticket.AIPriority = result.Priority
	ticket.AISentiment = result.Sentiment
	ticket.AITeam = result.RecommendedTeam
	ticket.AIConfidence = &result.Confidence
	ticket.AISummary = result.Summary
	ticket.AITags = result.Tags
	ticket.AIProcessingStatus = models.AIStatusCompleted
	ticket.ProcessedAt = &now

	if err := h.ticketRepo.UpdateAIFields(ticket); err != nil {
		return fmt.Errorf("failed to persist AI results: %w", err)
	}

	// Activity log
	_ = h.activityRepo.Create(&models.TicketActivity{
		TenantID:     tenantID,
		TicketID:     ticketID,
		UserID:       ticket.CreatedBy,
		ActivityType: models.ActivityAIAnalysisCompleted,
		NewValue:     result.Category,
		Description:  "AI analysis completed",
	})

	log.WithField("category", result.Category).
		WithField("confidence", result.Confidence).
		Info("AI Analysis: Completed successfully")
	return nil
}
