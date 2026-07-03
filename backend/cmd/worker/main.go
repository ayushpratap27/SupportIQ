package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ayush/supportiq/internal/ai/gemini"
	"github.com/ayush/supportiq/internal/ai/groq"
	"github.com/ayush/supportiq/internal/ai/provider"
	replyprovider "github.com/ayush/supportiq/internal/ai/reply/provider"
	"github.com/ayush/supportiq/internal/config"
	"github.com/ayush/supportiq/internal/database"
	"github.com/ayush/supportiq/internal/knowledge/retrieval"
	"github.com/ayush/supportiq/internal/models"
	"github.com/ayush/supportiq/internal/queue/redisqueue"
	"github.com/ayush/supportiq/internal/repositories"
	"github.com/ayush/supportiq/internal/services"
	"github.com/ayush/supportiq/internal/utils"
	workerhandlers "github.com/ayush/supportiq/worker/handlers"
	"github.com/ayush/supportiq/worker/processor"
)

func main() {
	// ─── Config ─────────────────────────────────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		utils.Logger.Fatalf("Worker: Failed to load config: %v", err)
	}

	if cfg.RedisURL == "" {
		utils.Logger.Fatal("Worker: REDIS_URL is required to run the worker")
	}

	utils.Logger.WithField("workers", cfg.WorkerCount).Info("Worker: Starting up")

	// ─── Database ────────────────────────────────────────────────────────────
	db, err := database.Connect(cfg)
	if err != nil {
		utils.Logger.Fatalf("Worker: Database connection failed: %v", err)
	}

	// ─── Redis queue ─────────────────────────────────────────────────────────
	redisQ, err := redisqueue.New(cfg.RedisURL, cfg.QueueName)
	if err != nil {
		utils.Logger.Fatalf("Worker: Redis connection failed: %v", err)
	}
	defer redisQ.Close()

	// ─── Repositories ────────────────────────────────────────────────────────
	ticketRepo := repositories.NewTicketRepository(db)
	activityRepo := repositories.NewActivityRepository(db)
	knowledgeRepo := repositories.NewKnowledgeRepository(db)
	replyRepo := repositories.NewReplyRepository(db)
	jobRepo := repositories.NewJobRepository(db)

	// ─── AI providers — priority: Groq (free) > Gemini > Noop ───────────────
	var aiProv provider.Provider
	var replyProv replyprovider.ReplyProvider
	activeModel := cfg.GeminiModel
	if cfg.GroqAPIKey != "" {
		groqClient := groq.NewClientWithReplyConfig(
			cfg.GroqAPIKey, cfg.GroqModel,
			time.Duration(cfg.AITimeout)*time.Second,
			cfg.AIMaxRetries, cfg.MaxReplyTokens, cfg.ReplyTemperature,
		)
		aiProv = groqClient
		replyProv = groqClient
		activeModel = cfg.GroqModel
		utils.Logger.WithField("model", cfg.GroqModel).Info("Worker: Groq provider initialised")
	} else if cfg.GeminiAPIKey != "" {
		geminiClient := gemini.NewClientWithReplyConfig(
			cfg.GeminiAPIKey, cfg.GeminiModel,
			time.Duration(cfg.AITimeout)*time.Second,
			cfg.AIMaxRetries, cfg.MaxReplyTokens, cfg.ReplyTemperature,
		)
		aiProv = geminiClient
		replyProv = geminiClient
		utils.Logger.WithField("model", cfg.GeminiModel).Info("Worker: Gemini provider initialised")
	} else {
		aiProv = &provider.NoopProvider{}
		replyProv = &replyprovider.NoopReplyProvider{}
		utils.Logger.Warn("Worker: No API key set — AI jobs will fail")
	}

	// ─── Services ────────────────────────────────────────────────────────────
	knowledgeRetriever := retrieval.NewPostgresRetriever(knowledgeRepo)
	replySvc := services.NewReplyService(replyProv, knowledgeRetriever, ticketRepo, replyRepo, activityRepo, activeModel)

	// ─── Job handlers ────────────────────────────────────────────────────────
	aiHandler := workerhandlers.NewAIAnalysisHandler(ticketRepo, activityRepo, aiProv)
	replyHandler := workerhandlers.NewGenerateReplyHandler(replySvc)

	// ─── Processor ───────────────────────────────────────────────────────────
	proc := processor.New(redisQ, redisQ, jobRepo, cfg.WorkerCount, cfg.MaxRetries, cfg.RetryDelay)
	proc.RegisterHandler(string(models.JobTypeAIAnalysis), aiHandler)
	proc.RegisterHandler(string(models.JobTypeGenerateReply), replyHandler)
	proc.RegisterHandler(string(models.JobTypeRegenerateReply), replyHandler)
	proc.RegisterHandler(string(models.JobTypeRetryAI), aiHandler)
	proc.RegisterHandler(string(models.JobTypeRetryReply), replyHandler)

	// ─── Graceful shutdown ───────────────────────────────────────────────────
	ctx, cancel := context.WithCancel(context.Background())

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-quit
		utils.Logger.WithField("signal", sig.String()).Info("Worker: Shutdown signal received")
		cancel()
	}()

	utils.Logger.WithField("queue", cfg.QueueName).
		WithField("workers", cfg.WorkerCount).
		Info("Worker: Ready — listening for jobs")

	proc.Start(ctx) // blocks until shutdown
	utils.Logger.Info("Worker: Exited cleanly")
}
