package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/ayush/supportiq/internal/ai/gemini"
	"github.com/ayush/supportiq/internal/ai/provider"
	replyprovider "github.com/ayush/supportiq/internal/ai/reply/provider"
	"github.com/ayush/supportiq/internal/analytics"
	analyticsreports "github.com/ayush/supportiq/internal/analytics/reports"
	"github.com/ayush/supportiq/internal/config"
	emailattachments "github.com/ayush/supportiq/internal/email/attachments"
	emailworkers "github.com/ayush/supportiq/internal/email/workers"
	"github.com/ayush/supportiq/internal/email/threading"
	"github.com/ayush/supportiq/internal/handlers"
	integrationspkg "github.com/ayush/supportiq/internal/integrations"
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
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SetupRouter wires together all middleware and route handlers and returns the engine.
func SetupRouter(db *gorm.DB, cfg *config.Config) *gin.Engine {
	router := gin.New()

	// Global middleware
	router.Use(gin.Recovery())
	router.Use(middleware.RequestLogger())
	router.Use(middleware.CORS())
	router.Use(middleware.SecurityHeaders())
	router.Use(middleware.RateLimit())
	// Cap request bodies at 2 MB to prevent large payload DoS.
	router.Use(func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2<<20)
		c.Next()
	})

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
		auth.POST("/register", middleware.RateLimitAuth(), authHandler.Register)
		auth.POST("/login", middleware.RateLimitAuth(), authHandler.Login)
		auth.POST("/logout", authHandler.Logout)
		auth.POST("/refresh", middleware.RateLimitAuth(), authHandler.Refresh)
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
			wsHub.ServeWS(c.Writer, c.Request, user.ID, user.TenantID)
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
			tenantRepo    := repositories.NewTenantRepository(db)

			// Email repositories
			emailAccountRepo  := repositories.NewEmailAccountRepository(db)
			emailMessageRepo  := repositories.NewEmailMessageRepository(db)

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

			// Knowledge retrieval (RAG) + core services
			knowledgeRetriever := retrieval.NewPostgresRetriever(knowledgeRepo)
			aiService     := services.NewAIService(aiProvider, ticketRepo, activityRepo)
			replyService  := services.NewReplyService(replyProvider, knowledgeRetriever, ticketRepo, replyRepo, activityRepo, cfg.GeminiModel)
			jobService    := services.NewJobService(jobRepo, jobQueue)
			aiService.SetReplyService(replyService)

			ticketService   := services.NewTicketService(ticketRepo, userRepo, activityRepo, aiService)
			ticketService.SetJobService(jobService)
					// SLA
					slaRepo    := repositories.NewSLARepository(db)
					slaSvc     := services.NewSLAService(slaRepo, ticketRepo, activityRepo, wsHub)
					slaHandler := handlers.NewSLAHandler(slaSvc)
					ticketService.SetSLAService(slaSvc)
					go slaSvc.StartMonitor(context.Background(), time.Minute)

				noteService     := services.NewNoteService(noteRepo, activityRepo)
			commentService  := services.NewCommentService(commentRepo, activityRepo)
				noteService.SetTicketRepo(ticketRepo)
				commentService.SetTicketRepo(ticketRepo)
				knowledgeService := services.NewKnowledgeService(knowledgeRepo)

				// Email services
				emailAccountSvc := services.NewEmailAccountService(emailAccountRepo, cfg.JWTAccessSecret)
				threadDetector  := threading.NewDetector(emailMessageRepo)
			attachStorage   := emailattachments.NewLocalStorage(cfg.AttachmentPath)
			emailSvc := services.NewEmailService(
				emailAccountRepo, emailMessageRepo,
				ticketRepo, activityRepo,
				emailAccountSvc, threadDetector,
				attachStorage, aiService, db,
			)
			emailSvc.SetJobService(jobService)

			// Start email background workers (context-less goroutines, stop on process exit)
			workerCtx := context.Background()
			pollInterval := time.Duration(cfg.EmailPollInterval) * time.Second
			go emailworkers.StartInboundWorker(workerCtx, emailAccountRepo, emailSvc, emailAccountSvc, pollInterval)
			go emailworkers.StartOutboundWorker(workerCtx, emailSvc, pollInterval, cfg.MaxEmailRetries)

			// Handlers
			ticketHandler    := handlers.NewTicketHandler(ticketService)
			noteHandler      := handlers.NewNoteHandler(noteService)
			commentHandler   := handlers.NewCommentHandler(commentService)
			activityHandler  := handlers.NewActivityHandler(activityRepo)
			aiHandler        := handlers.NewAIHandler(ticketRepo, aiService)
			replyHandler     := handlers.NewReplyHandler(replyService)
			replyHandler.SetEmailService(emailSvc) // wire email on approval
			knowledgeHandler := handlers.NewKnowledgeHandler(knowledgeService)
			jobHandler       := handlers.NewJobHandler(jobService)
			emailHandler     := handlers.NewEmailHandler(emailAccountSvc, emailSvc)

			_ = redisQ // suppress unused warning if queue unavailable

			// My tickets
			protected.GET("/my-tickets", ticketHandler.MyTickets)
			protected.GET("/activities", activityHandler.ListRecent)

			tickets := protected.Group("/tickets")
			{
				tickets.GET("/unassigned", ticketHandler.ListUnassigned)
				tickets.POST("",   ticketHandler.Create)
				tickets.GET("",    ticketHandler.List)
				tickets.GET("/sla", slaHandler.GetDashboard) // must be before /:id
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

				// Email thread for this ticket
				tickets.GET("/:id/emails",      emailHandler.GetTicketEmails)
				tickets.POST("/:id/send-email", emailHandler.SendEmail)
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

			// Email account management + monitor (admin only)
			emailGroup := protected.Group("/email", middleware.RequireRole(models.RoleAdmin))
			{
				emailGroup.GET("/accounts",           emailHandler.ListAccounts)
				emailGroup.POST("/accounts",          emailHandler.CreateAccount)
				emailGroup.PUT("/accounts/:id",       emailHandler.UpdateAccount)
				emailGroup.DELETE("/accounts/:id",    emailHandler.DeleteAccount)
				emailGroup.POST("/accounts/:id/test", emailHandler.TestConnection)
				emailGroup.GET("/monitor",            emailHandler.Monitor)
				emailGroup.POST("/sync",              emailHandler.Sync)
			}

			// Users
			userHandler := handlers.NewUserHandler(userRepo)
			users := protected.Group("/users")
			{
				users.GET("/agents", userHandler.ListAgents)
			}

			// Analytics — admins see everything; agents see personal metrics only
			analyticsRepo      := analytics.NewAnalyticsRepository(db)
				analyticsAggregator := analytics.NewAggregator(analyticsRepo, tenantRepo)
			analyticsSvc       := analytics.NewService(analyticsRepo, analyticsAggregator)
			reportSvc          := analyticsreports.NewService(db, cfg.ReportStoragePath, cfg.ReportRetentionDays)
			collector          := analyticsreports.NewDataCollector(db)
			analyticsHandler   := handlers.NewAnalyticsHandler(analyticsSvc, reportSvc, collector)

			// Start analytics scheduler
			aggInterval := time.Duration(cfg.AggregationInterval) * time.Second
			analyticsScheduler := analytics.NewScheduler(analyticsAggregator, analyticsSvc, wsHub, aggInterval)
			go analyticsScheduler.Start(context.Background())

			analyticsGroup := protected.Group("/analytics")
			{
				analyticsGroup.GET("/overview",  analyticsHandler.Overview)
				analyticsGroup.GET("/tickets",   analyticsHandler.TicketStats)
				analyticsGroup.GET("/agents",    analyticsHandler.AgentStats)
				analyticsGroup.GET("/ai",        analyticsHandler.AIStats)
				analyticsGroup.GET("/queues",    analyticsHandler.QueueStats)
				analyticsGroup.GET("/email",     analyticsHandler.EmailStats)
				analyticsGroup.GET("/trends",    analyticsHandler.Trends)

				// Report generation (all authenticated users; scope enforced in handler)
				analyticsGroup.POST("/reports",               analyticsHandler.GenerateReport)
				analyticsGroup.GET("/reports",                analyticsHandler.ListReports)
				analyticsGroup.GET("/reports/:id",            analyticsHandler.GetReport)
				analyticsGroup.GET("/reports/:id/download",   analyticsHandler.DownloadReport)

				// Manual aggregation trigger (admin only)
				analyticsGroup.POST("/aggregate",
					middleware.RequireRole(models.RoleAdmin),
					analyticsHandler.TriggerAggregation)
			}

			// Enterprise Integrations (admin only)
			integrationRepo     := repositories.NewIntegrationRepository(db)
			integrationRegistry := integrationspkg.NewRegistry()
			integrationSvc      := services.NewIntegrationService(integrationRepo, ticketRepo, integrationRegistry, cfg.JWTAccessSecret)
			integrationHandler  := handlers.NewIntegrationHandler(integrationSvc)

			// Start integration background worker
				integrationWorker := integrationspkg.NewWorker(integrationRepo, activityRepo, ticketRepo, tenantRepo, integrationRegistry, cfg.JWTAccessSecret)
			go integrationWorker.Start(context.Background())

			intGroup := protected.Group("/integrations", middleware.RequireRole(models.RoleAdmin))
			{
				intGroup.GET("",          integrationHandler.List)
				intGroup.POST("",         integrationHandler.Create)
				intGroup.PUT("/:id",      integrationHandler.Update)
				intGroup.DELETE("/:id",   integrationHandler.Delete)
				intGroup.POST("/:id/test",   integrationHandler.TestConnection)
				intGroup.GET("/:id/events",  integrationHandler.ListEvents)
			}

			// Ticket integration routes (all authenticated users)
			tickets.GET("/:id/integrations",       integrationHandler.GetTicketIntegrations)
			tickets.POST("/:id/create-jira",        integrationHandler.CreateJiraIssue)
			tickets.POST("/:id/create-linear",      integrationHandler.CreateLinearIssue)
			tickets.POST("/:id/create-github-issue", integrationHandler.CreateGitHubIssue)

					// SLA Policies (Admin only — manage policy CRUD)
					slaGroup := protected.Group("/sla-policies", middleware.RequireRole(models.RoleAdmin))
					{
						slaGroup.GET("",       slaHandler.ListPolicies)
						slaGroup.POST("",      slaHandler.CreatePolicy)
						slaGroup.GET("/:id",   slaHandler.GetPolicy)
						slaGroup.PUT("/:id",   slaHandler.UpdatePolicy)
						slaGroup.DELETE("/:id", slaHandler.DeletePolicy)
					}

					// Tenant Settings (current tenant admin)
						tenantSvc     := services.NewTenantService(tenantRepo)
						tenantHandler := handlers.NewTenantHandler(tenantSvc)
						settingsGroup := protected.Group("/settings")
						{
							settingsGroup.GET("",  tenantHandler.GetSettings)
							settingsGroup.PUT("",  middleware.RequireRole(models.RoleAdmin), tenantHandler.UpdateSettings)
						}

						// SuperAdmin routes
						adminGroup := protected.Group("/admin", middleware.RequireSuperAdmin())
						{
							adminGroup.GET("/tenants",        tenantHandler.List)
							adminGroup.POST("/tenants",       tenantHandler.Create)
							adminGroup.GET("/tenants/:id",    tenantHandler.GetByID)
							adminGroup.PUT("/tenants/:id",    tenantHandler.Update)
							adminGroup.DELETE("/tenants/:id", tenantHandler.Delete)
							adminGroup.GET("/overview",       tenantHandler.Overview)
						}
				}
	}

	return router
}

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
		// Route to the correct tenant if tenant_id is present in the payload
		if tidStr, ok := payload["tenant_id"].(string); ok && tidStr != "" {
			if tid, err := uuid.Parse(tidStr); err == nil && tid != uuid.Nil {
				hub.BroadcastToTenant(tid, payload)
				continue
			}
		}
		hub.Broadcast(payload)
	}
}
