package middleware

import (
	"net/http"
	"strings"

	"github.com/ayush/supportiq/internal/config"
	jwtpkg "github.com/ayush/supportiq/internal/jwt"
	"github.com/ayush/supportiq/internal/models"
	"github.com/ayush/supportiq/internal/utils"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Authenticate validates the Bearer token in the Authorization header,
// loads the corresponding user from the database, and stores the user's
// ID, role, and full record in the Gin context for downstream handlers.
func Authenticate(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			utils.SendError(c, http.StatusUnauthorized, "Authorization header missing or malformed")
			c.Abort()
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := jwtpkg.ValidateToken(tokenStr, cfg.JWTAccessSecret)
		if err != nil {
			utils.SendError(c, http.StatusUnauthorized, "Invalid or expired token")
			c.Abort()
			return
		}

		var user models.User
		if err := db.First(&user, claims.UserID).Error; err != nil {
			utils.SendError(c, http.StatusUnauthorized, "User not found")
			c.Abort()
			return
		}

		c.Set("userID", user.ID)
		c.Set("userRole", string(user.Role))
		c.Set("user", &user)
		c.Next()
	}
}
