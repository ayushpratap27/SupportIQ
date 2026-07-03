package middleware

import (
	"net/http"

	"github.com/ayush/supportiq/internal/models"
	"github.com/ayush/supportiq/internal/utils"
	"github.com/gin-gonic/gin"
)

// RequireRole returns middleware that permits access only to users whose role
// matches one of the provided values. Must be used after Authenticate.
//
// Example:
//
//	adminOnly := middleware.RequireRole(models.RoleAdmin)
//	agentOrAdmin := middleware.RequireRole(models.RoleAdmin, models.RoleSupportAgent)
func RequireRole(roles ...models.Role) gin.HandlerFunc {
	allowed := make(map[models.Role]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}

	return func(c *gin.Context) {
		roleVal, exists := c.Get("userRole")
		if !exists {
			utils.SendError(c, http.StatusForbidden, "Access denied")
			c.Abort()
			return
		}

		if _, ok := allowed[models.Role(roleVal.(string))]; !ok {
			utils.SendError(c, http.StatusForbidden, "Insufficient permissions")
			c.Abort()
			return
		}

		c.Next()
	}
}
