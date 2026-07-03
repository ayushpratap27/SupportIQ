package handlers

import (
	"net/http"

	"github.com/ayush/supportiq/internal/dto"
	"github.com/ayush/supportiq/internal/services"
	"github.com/ayush/supportiq/internal/utils"
	"github.com/ayush/supportiq/internal/validators"
	"github.com/gin-gonic/gin"
)

// AuthHandler exposes HTTP endpoints for authentication.
// It is intentionally thin — all business logic lives in AuthService.
type AuthHandler struct {
	authService *services.AuthService
}

// NewAuthHandler constructs an AuthHandler via dependency injection.
func NewAuthHandler(authService *services.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

// Register handles POST /api/v1/auth/register
func (h *AuthHandler) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := validators.ValidatePasswordStrength(req.Password); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error())
		return
	}

	resp, statusCode, err := h.authService.Register(&req)
	if err != nil {
		utils.SendError(c, statusCode, err.Error())
		return
	}

	utils.SendSuccess(c, http.StatusCreated, "Registration successful", resp)
}

// Login handles POST /api/v1/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error())
		return
	}

	resp, statusCode, err := h.authService.Login(&req)
	if err != nil {
		utils.SendError(c, statusCode, err.Error())
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Login successful", resp)
}

// Logout handles POST /api/v1/auth/logout
// JWT is stateless — the server-side action is a no-op; the client discards its token.
func (h *AuthHandler) Logout(c *gin.Context) {
	utils.SendSuccess(c, http.StatusOK, "Logged out successfully", nil)
}

// Me handles GET /api/v1/auth/me (requires Authenticate middleware)
func (h *AuthHandler) Me(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	resp, statusCode, err := h.authService.GetUserByID(userID.(uint))
	if err != nil {
		utils.SendError(c, statusCode, err.Error())
		return
	}

	utils.SendSuccess(c, http.StatusOK, "User retrieved", resp)
}
