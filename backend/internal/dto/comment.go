package dto

import (
	"time"

	"github.com/google/uuid"
)

type CreateCommentRequest struct {
	Message     string `json:"message"     binding:"required"`
	CommentType string `json:"commentType" binding:"omitempty,oneof=PUBLIC INTERNAL"`
}

type CommentResponse struct {
	ID          uint          `json:"id"`
	TicketID    uuid.UUID     `json:"ticket_id"`
	Message     string        `json:"message"`
	CommentType string        `json:"comment_type"`
	CreatedAt   time.Time     `json:"created_at"`
	User        *UserResponse `json:"user,omitempty"`
}
