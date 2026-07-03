package services

import (
	"errors"
	"fmt"
	"math"
	"net/http"

	"github.com/ayush/supportiq/internal/dto"
	"github.com/ayush/supportiq/internal/models"
	"github.com/ayush/supportiq/internal/repositories"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrTicketNotFound    = errors.New("ticket not found")
	ErrInvalidTransition = errors.New("invalid status transition")
	ErrForbidden         = errors.New("insufficient permissions")
	ErrAssigneeNotFound  = errors.New("assignee not found")
	ErrAssigneeNotAgent  = errors.New("assignee must have SupportAgent role")
	ErrAlreadyAssigned   = errors.New("ticket is already assigned")
)

// TicketService contains all business logic for ticket management.
type TicketService struct {
	repo         *repositories.TicketRepository
	userRepo     *repositories.UserRepository
	activityRepo *repositories.ActivityRepository
	aiService    *AIService  // goroutine-based fallback when no queue
	jobSvc       *JobService // Redis-backed queue (preferred when available)
}

func NewTicketService(
	repo *repositories.TicketRepository,
	userRepo *repositories.UserRepository,
	activityRepo *repositories.ActivityRepository,
	aiService *AIService,
) *TicketService {
	return &TicketService{repo: repo, userRepo: userRepo, activityRepo: activityRepo, aiService: aiService}
}

// SetJobService injects the job service for Redis-backed async processing.
// Must be called after both TicketService and JobService are constructed.
func (s *TicketService) SetJobService(js *JobService) {
	s.jobSvc = js
}

// logActivity is fire-and-forget; errors are silently suppressed so they never
// fail the main operation.
func (s *TicketService) logActivity(ticketID uuid.UUID, userID uint, actType, oldVal, newVal, desc string) {
	_ = s.activityRepo.Create(&models.TicketActivity{
		TicketID:     ticketID,
		UserID:       userID,
		ActivityType: actType,
		OldValue:     oldVal,
		NewValue:     newVal,
		Description:  desc,
	})
}

// Create inserts a new ticket, generating its sequential number inside a transaction.
func (s *TicketService) Create(req *dto.CreateTicketRequest, createdBy uint) (*dto.TicketResponse, int, error) {
	var created models.Ticket

	err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		ticketNum, err := s.repo.NextTicketNumber(tx)
		if err != nil {
			return err
		}
		created = models.Ticket{
			Subject:       req.Subject,
			Description:   req.Description,
			CustomerName:  req.CustomerName,
			CustomerEmail: req.CustomerEmail,
			TicketNumber:  ticketNum,
			Status:        models.TicketStatusOpen,
			Priority:      models.TicketPriorityMedium,
			Category:      models.TicketCategoryGeneral,
			Source:        models.TicketSourceWeb,
			CreatedBy:     createdBy,
		}
		return s.repo.Create(tx, &created)
	})
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	s.logActivity(created.ID, createdBy, models.ActivityCreateTicket, "", "", "Ticket created")

	// Trigger AI analysis — use Redis queue if available, otherwise fall back to goroutine
	if s.jobSvc != nil && s.jobSvc.IsQueueAvailable() {
		_ = s.jobSvc.EnqueueAIAnalysis(created.ID, createdBy)
	} else {
		s.aiService.AnalyzeTicket(created.ID)
	}

	full, err := s.repo.FindByID(created.ID)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	return toTicketResponse(full), http.StatusCreated, nil
}

// List returns a paginated, filtered, and searched set of tickets.
func (s *TicketService) List(q *dto.ListTicketsQuery) (*dto.ListTicketsResponse, int, error) {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.Limit < 1 || q.Limit > 100 {
		q.Limit = 20
	}

	tickets, total, err := s.repo.List(q)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	items := make([]dto.TicketResponse, len(tickets))
	for i := range tickets {
		items[i] = *toTicketResponse(&tickets[i])
	}

	totalPages := int(math.Ceil(float64(total) / float64(q.Limit)))
	if totalPages == 0 {
		totalPages = 1
	}

	return &dto.ListTicketsResponse{
		Items:       items,
		TotalCount:  total,
		CurrentPage: q.Page,
		TotalPages:  totalPages,
		Limit:       q.Limit,
	}, http.StatusOK, nil
}

// GetByID returns a single ticket by its UUID.
func (s *TicketService) GetByID(id uuid.UUID) (*dto.TicketResponse, int, error) {
	t, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, http.StatusNotFound, ErrTicketNotFound
		}
		return nil, http.StatusInternalServerError, err
	}
	return toTicketResponse(t), http.StatusOK, nil
}

// Update applies editable field changes and logs relevant activity entries.
func (s *TicketService) Update(id uuid.UUID, req *dto.UpdateTicketRequest, userID uint) (*dto.TicketResponse, int, error) {
	t, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, http.StatusNotFound, ErrTicketNotFound
		}
		return nil, http.StatusInternalServerError, err
	}

	oldPriority := string(t.Priority)
	oldCategory := string(t.Category)

	if req.Subject != "" {
		t.Subject = req.Subject
	}
	if req.Description != "" {
		t.Description = req.Description
	}
	if req.Priority != "" {
		t.Priority = models.TicketPriority(req.Priority)
	}
	if req.Category != "" {
		t.Category = models.TicketCategory(req.Category)
	}
	if req.CustomerName != "" {
		t.CustomerName = req.CustomerName
	}
	if req.CustomerEmail != "" {
		t.CustomerEmail = req.CustomerEmail
	}

	if err := s.repo.Update(t); err != nil {
		return nil, http.StatusInternalServerError, err
	}

	if req.Priority != "" && string(t.Priority) != oldPriority {
		s.logActivity(id, userID, models.ActivityPriorityChanged, oldPriority, string(t.Priority),
			fmt.Sprintf("Priority changed from %s to %s", oldPriority, string(t.Priority)))
	}
	if req.Category != "" && string(t.Category) != oldCategory {
		s.logActivity(id, userID, models.ActivityCategoryChanged, oldCategory, string(t.Category),
			fmt.Sprintf("Category changed from %s to %s", oldCategory, string(t.Category)))
	}
	if req.Subject != "" || req.Description != "" || req.CustomerName != "" || req.CustomerEmail != "" {
		s.logActivity(id, userID, models.ActivityUpdateTicket, "", "", "Ticket details updated")
	}

	// Re-trigger AI analysis when the ticket content changes
	if req.Subject != "" || req.Description != "" {
		if s.jobSvc != nil && s.jobSvc.IsQueueAvailable() {
			_ = s.jobSvc.EnqueueAIAnalysis(id, userID)
		} else {
			s.aiService.AnalyzeTicket(id)
		}
	}

	return toTicketResponse(t), http.StatusOK, nil
}

// UpdateStatus transitions the ticket to a new status, enforcing the linear workflow.
func (s *TicketService) UpdateStatus(id uuid.UUID, req *dto.UpdateStatusRequest, userID uint) (*dto.TicketResponse, int, error) {
	t, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, http.StatusNotFound, ErrTicketNotFound
		}
		return nil, http.StatusInternalServerError, err
	}

	oldStatus := string(t.Status)
	newStatus := models.TicketStatus(req.Status)
	if !models.IsValidStatusTransition(t.Status, newStatus) {
		return nil, http.StatusBadRequest, ErrInvalidTransition
	}

	t.Status = newStatus
	if err := s.repo.Update(t); err != nil {
		return nil, http.StatusInternalServerError, err
	}

	actType := models.ActivityStatusChanged
	if newStatus == models.TicketStatusClosed {
		actType = models.ActivityTicketClosed
	}
	s.logActivity(id, userID, actType, oldStatus, string(newStatus),
		fmt.Sprintf("Status changed from %s to %s", oldStatus, string(newStatus)))

	return toTicketResponse(t), http.StatusOK, nil
}

// Assign sets the assigned_to field. Only Admins may call this; assignee must be a SupportAgent.
func (s *TicketService) Assign(id uuid.UUID, req *dto.AssignTicketRequest, userID uint, userRole string) (*dto.TicketResponse, int, error) {
	if models.Role(userRole) != models.RoleAdmin {
		return nil, http.StatusForbidden, ErrForbidden
	}

	t, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, http.StatusNotFound, ErrTicketNotFound
		}
		return nil, http.StatusInternalServerError, err
	}

	assignee, err := s.userRepo.FindByID(req.AssignedTo)
	if err != nil {
		return nil, http.StatusNotFound, ErrAssigneeNotFound
	}
	if assignee.Role != models.RoleSupportAgent {
		return nil, http.StatusBadRequest, ErrAssigneeNotAgent
	}

	t.AssignedTo = &req.AssignedTo
	if err := s.repo.Update(t); err != nil {
		return nil, http.StatusInternalServerError, err
	}

	s.logActivity(id, userID, models.ActivityAssignTicket, "", assignee.Name,
		fmt.Sprintf("Assigned to %s", assignee.Name))

	return toTicketResponse(t), http.StatusOK, nil
}

// Delete soft-deletes a ticket. Only Admins may delete.
func (s *TicketService) Delete(id uuid.UUID, userRole string) (int, error) {
	if models.Role(userRole) != models.RoleAdmin {
		return http.StatusForbidden, ErrForbidden
	}

	if _, err := s.repo.FindByID(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return http.StatusNotFound, ErrTicketNotFound
		}
		return http.StatusInternalServerError, err
	}

	if err := s.repo.SoftDelete(id); err != nil {
		return http.StatusInternalServerError, err
	}
	return http.StatusOK, nil
}

// TakeOwnership lets a SupportAgent claim an unassigned ticket.
func (s *TicketService) TakeOwnership(id uuid.UUID, userID uint, userRole string) (*dto.TicketResponse, int, error) {
	if models.Role(userRole) != models.RoleSupportAgent {
		return nil, http.StatusForbidden, ErrForbidden
	}

	t, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, http.StatusNotFound, ErrTicketNotFound
		}
		return nil, http.StatusInternalServerError, err
	}

	if t.AssignedTo != nil {
		return nil, http.StatusConflict, ErrAlreadyAssigned
	}

	t.AssignedTo = &userID
	if err := s.repo.Update(t); err != nil {
		return nil, http.StatusInternalServerError, err
	}

	s.logActivity(id, userID, models.ActivityTakeOwnership, "", "", "Took ownership of ticket")

	full, _ := s.repo.FindByID(id)
	if full == nil {
		return toTicketResponse(t), http.StatusOK, nil
	}
	return toTicketResponse(full), http.StatusOK, nil
}

// MyTickets returns tickets assigned to the given user.
func (s *TicketService) MyTickets(userID uint, q *dto.ListTicketsQuery) (*dto.ListTicketsResponse, int, error) {
	q.AssignedTo = &userID
	q.UnassignedOnly = false
	return s.List(q)
}

// ListUnassigned returns tickets where assigned_to IS NULL.
func (s *TicketService) ListUnassigned(q *dto.ListTicketsQuery) (*dto.ListTicketsResponse, int, error) {
	q.UnassignedOnly = true
	q.AssignedTo = nil
	return s.List(q)
}

func toTicketResponse(t *models.Ticket) *dto.TicketResponse {
	resp := &dto.TicketResponse{
		ID:                 t.ID,
		TicketNumber:       t.TicketNumber,
		Subject:            t.Subject,
		Description:        t.Description,
		Status:             string(t.Status),
		Priority:           string(t.Priority),
		Category:           string(t.Category),
		Source:             string(t.Source),
		AssignedTo:         t.AssignedTo,
		CreatedBy:          t.CreatedBy,
		CustomerName:       t.CustomerName,
		CustomerEmail:      t.CustomerEmail,
		CreatedAt:          t.CreatedAt,
		UpdatedAt:          t.UpdatedAt,
		AICategory:         t.AICategory,
		AIPriority:         t.AIPriority,
		AISentiment:        t.AISentiment,
		AITeam:             t.AITeam,
		AIConfidence:       t.AIConfidence,
		AISummary:          t.AISummary,
		AITags:             t.AITags,
		AIProcessingStatus: t.AIProcessingStatus,
		ProcessedAt:        t.ProcessedAt,
	}

	if t.Creator != nil {
		ur := dto.UserResponse{ID: t.Creator.ID, Name: t.Creator.Name, Email: t.Creator.Email, Role: string(t.Creator.Role)}
		resp.Creator = &ur
	}
	if t.Assignee != nil {
		ur := dto.UserResponse{ID: t.Assignee.ID, Name: t.Assignee.Name, Email: t.Assignee.Email, Role: string(t.Assignee.Role)}
		resp.Assignee = &ur
	}
	return resp
}
