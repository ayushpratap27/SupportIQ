package handlers

import (
	"net/http"

	"github.com/ayush/supportiq/internal/dto"
	"github.com/ayush/supportiq/internal/services"
	"github.com/ayush/supportiq/internal/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// TicketHandler is a thin HTTP layer that validates input, delegates to the
// TicketService, and writes a consistent JSON response.
type TicketHandler struct {
	service *services.TicketService
}

func NewTicketHandler(service *services.TicketService) *TicketHandler {
	return &TicketHandler{service: service}
}

// Create handles POST /api/v1/tickets
func (h *TicketHandler) Create(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	var req dto.CreateTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error())
		return
	}

	resp, statusCode, err := h.service.Create(&req, userID)
	if err != nil {
		utils.SendError(c, statusCode, err.Error())
		return
	}
	utils.SendSuccess(c, http.StatusCreated, "Ticket created successfully", resp)
}

// List handles GET /api/v1/tickets
func (h *TicketHandler) List(c *gin.Context) {
	var q dto.ListTicketsQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error())
		return
	}

	resp, statusCode, err := h.service.List(&q)
	if err != nil {
		utils.SendError(c, statusCode, err.Error())
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Tickets retrieved", resp)
}

// GetByID handles GET /api/v1/tickets/:id
func (h *TicketHandler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid ticket ID")
		return
	}

	resp, statusCode, err := h.service.GetByID(id)
	if err != nil {
		utils.SendError(c, statusCode, err.Error())
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Ticket retrieved", resp)
}

// Update handles PUT /api/v1/tickets/:id
func (h *TicketHandler) Update(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid ticket ID")
		return
	}

	var req dto.UpdateTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error())
		return
	}

	resp, statusCode, err := h.service.Update(id, &req, userID)
	if err != nil {
		utils.SendError(c, statusCode, err.Error())
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Ticket updated successfully", resp)
}

// UpdateStatus handles PATCH /api/v1/tickets/:id/status
func (h *TicketHandler) UpdateStatus(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid ticket ID")
		return
	}

	var req dto.UpdateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error())
		return
	}

	resp, statusCode, err := h.service.UpdateStatus(id, &req, userID)
	if err != nil {
		utils.SendError(c, statusCode, err.Error())
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Status updated successfully", resp)
}

// Assign handles PATCH /api/v1/tickets/:id/assign
func (h *TicketHandler) Assign(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	userRole := c.MustGet("userRole").(string)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid ticket ID")
		return
	}

	var req dto.AssignTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error())
		return
	}

	resp, statusCode, err := h.service.Assign(id, &req, userID, userRole)
	if err != nil {
		utils.SendError(c, statusCode, err.Error())
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Ticket assigned successfully", resp)
}

// Delete handles DELETE /api/v1/tickets/:id
func (h *TicketHandler) Delete(c *gin.Context) {
	userRole := c.MustGet("userRole").(string)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid ticket ID")
		return
	}

	statusCode, err := h.service.Delete(id, userRole)
	if err != nil {
		utils.SendError(c, statusCode, err.Error())
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Ticket deleted successfully", nil)
}

// TakeOwnership handles PATCH /api/v1/tickets/:id/take-ownership
func (h *TicketHandler) TakeOwnership(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	userRole := c.MustGet("userRole").(string)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid ticket ID")
		return
	}

	resp, statusCode, err := h.service.TakeOwnership(id, userID, userRole)
	if err != nil {
		utils.SendError(c, statusCode, err.Error())
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Ticket ownership taken successfully", resp)
}

// MyTickets handles GET /api/v1/my-tickets
func (h *TicketHandler) MyTickets(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	var q dto.ListTicketsQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error())
		return
	}

	resp, statusCode, err := h.service.MyTickets(userID, &q)
	if err != nil {
		utils.SendError(c, statusCode, err.Error())
		return
	}
	utils.SendSuccess(c, http.StatusOK, "My tickets retrieved", resp)
}

// ListUnassigned handles GET /api/v1/tickets/unassigned
func (h *TicketHandler) ListUnassigned(c *gin.Context) {
	var q dto.ListTicketsQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error())
		return
	}

	resp, statusCode, err := h.service.ListUnassigned(&q)
	if err != nil {
		utils.SendError(c, statusCode, err.Error())
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Unassigned tickets retrieved", resp)
}
