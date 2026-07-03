package routes

import (
	"time"

	"github.com/ayush/supportiq/internal/ai/gemini"
	"github.com/ayush/supportiq/internal/ai/provider"
	"github.com/ayush/supportiq/internal/config"
	"github.com/ayush/supportiq/internal/handlers"
	"github.com/ayush/supportiq/internal/middleware"
	"github.com/ayush/supportiq/internal/repositories"
	"github.com/ayush/supportiq/internal/services"
	"github.com/ayush/supportiq/internal/utils"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SetupRouter wires together all middleware and route handlers and returns the engine.
func SetupRouter(db *gorm.DB, cfg *config.Config) *gin.Engine {
	router := gin.New()

	// Global middleware
	router.Use(gin.Recovery())
	router.Use(middleware.RequestLogger())
	router.Use(middleware.CORS())

	// API v1 group
	api := router.Group("/api/v1")
	{
		// Public — health check
		healthHandler := handlers.NewHealthHandler()
		api.GET("/health", healthHandler.Check)

		// Auth routes
		authService := services.NewAuthService(db, cfg)
		authHandler := handlers.NewAuthHandler(authService)

		auth := api.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
			auth.POST("/logout", authHandler.Logout)
			auth.GET("/me", middleware.Authenticate(db, cfg), authHandler.Me)
		}

		// All routes below require a valid JWT
		protected := api.Group("", middleware.Authenticate(db, cfg))
		{
			// Shared repositories
			ticketRepo := repositories.NewTicketRepository(db)
			userRepo := repositories.NewUserRepository(db)
			activityRepo := repositories.NewActivityRepository(db)
			noteRepo := repositories.NewNoteRepository(db)
			commentRepo := repositories.NewCommentRepository(db)

			// AI provider — falls back to NoopProvider when no key is configured
			var aiProvider provider.Provider
			if cfg.GeminiAPIKey != "" {
				aiProvider = gemini.NewClient(
					cfg.GeminiAPIKey,
					cfg.GeminiModel,
					time.Duration(cfg.AITimeout)*time.Second,
					cfg.AIMaxRetries,
				)
				utils.Logger.WithField("model", cfg.GeminiModel).Info("AI: Gemini provider initialised")
			} else {
				aiProvider = &provider.NoopProvider{}
				utils.Logger.Warn("AI: GEMINI_API_KEY not set — AI analysis will be disabled")
			}

			// Services
			aiService := services.NewAIService(aiProvider, ticketRepo, activityRepo)
			ticketService := services.NewTicketService(ticketRepo, userRepo, activityRepo, aiService)
			noteService := services.NewNoteService(noteRepo, activityRepo)
			commentService := services.NewCommentService(commentRepo, activityRepo)

			// Handlers
			ticketHandler := handlers.NewTicketHandler(ticketService)
			noteHandler := handlers.NewNoteHandler(noteService)
			commentHandler := handlers.NewCommentHandler(commentService)
			activityHandler := handlers.NewActivityHandler(activityRepo)
			aiHandler := handlers.NewAIHandler(ticketRepo, aiService)

			// My tickets (flat path)
			protected.GET("/my-tickets", ticketHandler.MyTickets)

			// Global recent activity feed (for dashboard)
			protected.GET("/activities", activityHandler.ListRecent)

			tickets := protected.Group("/tickets")
			{
				// Static sub-paths registered before /:id to avoid conflicts
				tickets.GET("/unassigned", ticketHandler.ListUnassigned)

				tickets.POST("", ticketHandler.Create)
				tickets.GET("", ticketHandler.List)
				tickets.GET("/:id", ticketHandler.GetByID)
				tickets.PUT("/:id", ticketHandler.Update)
				tickets.PATCH("/:id/status", ticketHandler.UpdateStatus)
				tickets.PATCH("/:id/assign", ticketHandler.Assign)
				tickets.PATCH("/:id/take-ownership", ticketHandler.TakeOwnership)
				tickets.DELETE("/:id", ticketHandler.Delete)

				// Per-ticket sub-resources
				tickets.POST("/:id/notes", noteHandler.Create)
				tickets.GET("/:id/notes", noteHandler.List)
				tickets.POST("/:id/comments", commentHandler.Create)
				tickets.GET("/:id/comments", commentHandler.List)
				tickets.GET("/:id/activity", activityHandler.ListByTicket)

				// AI analysis
				tickets.GET("/:id/ai-analysis", aiHandler.GetAnalysis)
				tickets.POST("/:id/retry-ai", aiHandler.RetryAnalysis)
			}

			// User utility routes
			userHandler := handlers.NewUserHandler(userRepo)
			users := protected.Group("/users")
			{
				users.GET("/agents", userHandler.ListAgents)
			}
		}
	}

	return router
}
