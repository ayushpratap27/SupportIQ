package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/ayush/supportiq/internal/ai/gemini"
	"github.com/ayush/supportiq/internal/ai/provider"
	replyprovider "github.com/ayush/supportiq/internal/ai/reply/provider"
	"github.com/ayush/supportiq/internal/config"
	"github.com/ayush/supportiq/internal/handlers"
	"github.com/ayush/supportiq/internal/knowledge/retrieval"
	jwtpkg "github.com/ayush/supportiq/internal/jwt"
	"github.com/ayush/supportiq/internal/middleware"
	"github.com/ayush/supportiq/internal/models"
	"github.com/ayush/supportiq/internal/queue"
	"github.com/ayush/supportiq/internal/queue/redisqueue"
	"github.com/ayush/supportiq/internal/repositories"
	"github.com/ayush/supportiq/internal/services"
	"github.com/ayush/supportiq/internal/utils"
	appws "github.com/ayush/supportiq/internal/websocket"
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

	// ─── WebSocket hub ────────────────────────────────────────────────────────
	wsHub := appws.NewHub()
	go wsHub.Run()

	// ─── Redis queue (optional — graceful degradation) ────────────────────────
	var redisQ *redisqueue.Client
	var jobQueue queue.Queue

	if cfg.RedisURL != "" {
		rq, err := redisqueue.New(cfg.RedisURL, cfg.QueueName)
		if err != nil {
			utils.Logger.WithError(err).Warn("Routes: Redis connection failed — running without job queue")
		} else {
			redisQ = rq
			jobQueue = rq
			utils.Logger.Info("Routes: Redis queue connected")
			// Subscribe to worker events and forward to WebSocket clients
			go subscribeToWorkerEvents(redisQ, wsHub)
		}
	} else {
		utils.Logger.Warn("Routes: REDIS_URL not set — job queue disabled, falling back to goroutines")
	}

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

		// WebSocket endpoint — authenticated via JWT query param
		api.GET("/ws", func(c *gin.Context) {
			token := c.Query("token")
			if token == "" {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "token required"})
				return
			}
			claims, err := jwtpkg.ValidateToken(token, cfg.JWTAccessSecret)
			if err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
				return
			}
			var user models.User
			if err := db.First(&user, claims.UserID).Error; err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
				return
			}
			wsHub.ServeWS(c.Writer, c.Request, user.ID)
		})

		// All routes below require a valid JWT
		protected := api.Group("", middleware.Authenticate(db, cfg))
		{
			// Shared repositories
			ticketRepo    := repositories.NewTicketRepository(db)
			userRepo      := repositories.NewUserRepository(db)
			activityRepo  := repositories.NewActivityRepository(db)
			noteRepo      := repositories.NewNoteRepository(db)
			commentRepo   := repositories.NewCommentRepository(db)
			knowledgeRepo := repositories.NewKnowledgeRepository(db)
			replyRepo     := repositories.NewReplyRepository(db)
			jobRepo       := repositories.NewJobRepository(db)

			// AI providers
			var aiProvider provider.Provider
			var replyProvider replyprovider.ReplyProvider
			if cfg.GeminiAPIKey != "" {
				geminiClient := gemini.NewClientWithReplyConfig(
					cfg.GeminiAPIKey,
					cfg.GeminiModel,
					time.Duration(cfg.AITimeout)*time.Second,
					cfg.AIMaxRetries,
					cfg.MaxReplyTokens,
					cfg.ReplyTemperature,
				)
				aiProvider    = geminiClient
				replyProvider = geminiClient
				utils.Logger.WithField("model", cfg.GeminiModel).Info("AI: Gemini provider initialised")
			} else {
				aiProvider    = &provider.NoopProvider{}
				replyProvider = &replyprovider.NoopReplyProvider{}
				utils.Logger.Warn("AI: GEMINI_API_KEY not set — AI features will be disabled")
			}

			// Knowledge retrieval (RAG) + Services
			knowledgeRetriever := retrieval.NewPostgresRetriever(knowledgeRepo)
			aiService     := services.NewAIService(aiProvider, ticketRepo, activityRepo)
			replyService  := services.NewReplyService(replyProvider, knowledgeRetriever, ticketRepo, replyRepo, activityRepo, cfg.GeminiModel)
			jobService    := services.NewJobService(jobRepo, jobQueue)
			aiService.SetReplyService(replyService) // goroutine fallback chain

			ticketService   := services.NewTicketService(ticketRepo, userRepo, activityRepo, aiService)
			ticketService.SetJobService(jobService)  // prefer queue over goroutine
			noteService     := services.NewNoteService(noteRepo, activityRepo)
			commentService  := services.NewCommentService(commentRepo, activityRepo)
			knowledgeService := services.NewKnowledgeService(knowledgeRepo)

			// Handlers
			ticketHandler    := handlers.NewTicketHandler(ticketService)
			noteHandler      := handlers.NewNoteHandler(noteService)
			commentHandler   := handlers.NewCommentHandler(commentService)
			activityHandler  := handlers.NewActivityHandler(activityRepo)
			aiHandler        := handlers.NewAIHandler(ticketRepo, aiService)
			replyHandler     := handlers.NewReplyHandler(replyService)
			knowledgeHandler := handlers.NewKnowledgeHandler(knowledgeService)
			jobHandler       := handlers.NewJobHandler(jobService)

			_ = redisQ // suppress unused warning if queue unavailable

			// My tickets
			protected.GET("/my-tickets", ticketHandler.MyTickets)
			protected.GET("/activities", activityHandler.ListRecent)

			tickets := protected.Group("/tickets")
			{
				tickets.GET("/unassigned", ticketHandler.ListUnassigned)
				tickets.POST("",   ticketHandler.Create)
				tickets.GET("",    ticketHandler.List)
				tickets.GET("/:id",  ticketHandler.GetByID)
				tickets.PUT("/:id",  ticketHandler.Update)
				tickets.PATCH("/:id/status",         ticketHandler.UpdateStatus)
				tickets.PATCH("/:id/assign",         ticketHandler.Assign)
				tickets.PATCH("/:id/take-ownership", ticketHandler.TakeOwnership)
				tickets.DELETE("/:id",               ticketHandler.Delete)

				tickets.POST("/:id/notes",    noteHandler.Create)
				tickets.GET("/:id/notes",     noteHandler.List)
				tickets.POST("/:id/comments", commentHandler.Create)
				tickets.GET("/:id/comments",  commentHandler.List)
				tickets.GET("/:id/activity",  activityHandler.ListByTicket)

				tickets.GET("/:id/ai-analysis",  aiHandler.GetAnalysis)
				tickets.POST("/:id/retry-ai",    aiHandler.RetryAnalysis)

				tickets.GET("/:id/reply",               replyHandler.GetReply)
				tickets.POST("/:id/reply/generate",     replyHandler.GenerateReply)
				tickets.POST("/:id/reply/regenerate",   replyHandler.RegenerateReply)
				tickets.PATCH("/:id/reply/edit",        replyHandler.EditReply)
				tickets.POST("/:id/reply/approve",
					middleware.RequireRole(models.RoleAdmin, models.RoleSupportAgent),
					replyHandler.ApproveReply)
				tickets.POST("/:id/reply/reject", replyHandler.RejectReply)
			}

			// Knowledge base (admin only)
			kb := protected.Group("/knowledge-base", middleware.RequireRole(models.RoleAdmin))
			{
				kb.GET("",      knowledgeHandler.List)
				kb.POST("",     knowledgeHandler.Create)
				kb.PUT("/:id",  knowledgeHandler.Update)
				kb.DELETE("/:id", knowledgeHandler.Delete)
			}

			// Job monitoring (admin only)
			jobs := protected.Group("/jobs", middleware.RequireRole(models.RoleAdmin))
			{
				jobs.GET("",                jobHandler.List)
				jobs.GET("/statistics",     jobHandler.Statistics)
				jobs.GET("/:id",            jobHandler.GetByID)
				jobs.POST("/:id/retry",     jobHandler.Retry)
			}

			// Users
			userHandler := handlers.NewUserHandler(userRepo)
			users := protected.Group("/users")
			{
				users.GET("/agents", userHandler.ListAgents)
			}
		}
	}

	return router
}

// subscribeToWorkerEvents listens to the Redis pub/sub channel and broadcasts
// all events to WebSocket clients connected to the hub.
func subscribeToWorkerEvents(rq *redisqueue.Client, hub *appws.Hub) {
	sub := rq.Subscribe(context.Background())
	defer sub.Close()

	ch := sub.Channel()
	for msg := range ch {
		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(msg.Payload), &payload); err != nil {
			utils.Logger.WithError(err).Warn("WS: failed to parse worker event")
			continue
		}
		hub.Broadcast(payload)
	}
}
