package utils

import (
	"github.com/gin-gonic/gin"
)

// successResponse is the standard envelope for successful API responses.
type successResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// errorResponse is the standard envelope for error API responses.
type errorResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// SendSuccess writes a JSON success response with the given status code.
func SendSuccess(c *gin.Context, statusCode int, message string, data interface{}) {
	c.JSON(statusCode, successResponse{
		Status:  "success",
		Message: message,
		Data:    data,
	})
}

// SendError writes a JSON error response with the given status code.
func SendError(c *gin.Context, statusCode int, message string) {
	c.JSON(statusCode, errorResponse{
		Status:  "error",
		Message: message,
	})
}
