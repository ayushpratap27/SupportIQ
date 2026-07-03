package services

import (
	"net/http"

	"github.com/ayush/supportiq/internal/dto"
	"github.com/ayush/supportiq/internal/models"
	"github.com/ayush/supportiq/internal/repositories"
	"github.com/google/uuid"
)

// CommentService handles business logic for ticket comments.
type CommentService struct {
	commentRepo  *repositories.CommentRepository
	activityRepo *repositories.ActivityRepository
}

func NewCommentService(commentRepo *repositories.CommentRepository, activityRepo *repositories.ActivityRepository) *CommentService {
	return &CommentService{commentRepo: commentRepo, activityRepo: activityRepo}
}

func (s *CommentService) Create(ticketID uuid.UUID, req *dto.CreateCommentRequest, userID uint) (*dto.CommentResponse, int, error) {
	commentType := models.CommentTypePublic
	if req.CommentType == "INTERNAL" {
		commentType = models.CommentTypeInternal
	}

	comment := &models.TicketComment{
		TicketID:    ticketID,
		UserID:      userID,
		Message:     req.Message,
		CommentType: commentType,
	}
	if err := s.commentRepo.Create(comment); err != nil {
		return nil, http.StatusInternalServerError, err
	}

	// Reload with user association
	loaded, err := s.commentRepo.FindByID(comment.ID)
	if err != nil {
		r := toCommentResponse(comment)
		return &r, http.StatusCreated, nil
	}

	_ = s.activityRepo.Create(&models.TicketActivity{
		TicketID:     ticketID,
		UserID:       userID,
		ActivityType: models.ActivityCommentAdded,
		Description:  "Comment added",
	})

	r := toCommentResponse(loaded)
	return &r, http.StatusCreated, nil
}

func (s *CommentService) List(ticketID uuid.UUID) ([]dto.CommentResponse, int, error) {
	comments, err := s.commentRepo.ListByTicketID(ticketID)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	responses := make([]dto.CommentResponse, len(comments))
	for i := range comments {
		responses[i] = toCommentResponse(&comments[i])
	}
	return responses, http.StatusOK, nil
}

func toCommentResponse(c *models.TicketComment) dto.CommentResponse {
	resp := dto.CommentResponse{
		ID:          c.ID,
		TicketID:    c.TicketID,
		Message:     c.Message,
		CommentType: string(c.CommentType),
		CreatedAt:   c.CreatedAt,
	}
	if c.User != nil {
		ur := dto.UserResponse{ID: c.User.ID, Name: c.User.Name, Email: c.User.Email, Role: string(c.User.Role)}
		resp.User = &ur
	}
	return resp
}
