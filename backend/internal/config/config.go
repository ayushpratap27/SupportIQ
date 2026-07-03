package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	Port             string
	DatabaseURL      string
	Environment      string
	JWTAccessSecret  string
	JWTRefreshSecret string

	// AI configuration
	GeminiAPIKey string
	GeminiModel  string
	AITimeout    int // seconds
	AIMaxRetries int
}

// Load reads environment variables (from .env in development) and returns a Config.
// Returns an error if any required variable is missing.
func Load() (*Config, error) {
	// Load .env file if present; silently ignored in production
	_ = godotenv.Load()

	cfg := &Config{
		Port:             getEnv("PORT", "8080"),
		DatabaseURL:      getEnv("DATABASE_URL", ""),
		Environment:      getEnv("APP_ENV", "development"),
		JWTAccessSecret:  getEnv("JWT_ACCESS_SECRET", ""),
		JWTRefreshSecret: getEnv("JWT_REFRESH_SECRET", ""),
		GeminiAPIKey:     getEnv("GEMINI_API_KEY", ""),
		GeminiModel:      getEnv("GEMINI_MODEL", "gemini-2.0-flash"),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is required")
	}
	if cfg.JWTAccessSecret == "" {
		return nil, fmt.Errorf("JWT_ACCESS_SECRET environment variable is required")
	}
	if cfg.JWTRefreshSecret == "" {
		return nil, fmt.Errorf("JWT_REFRESH_SECRET environment variable is required")
	}

	if v := getEnv("AI_TIMEOUT", ""); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.AITimeout = n
		}
	}
	if cfg.AITimeout == 0 {
		cfg.AITimeout = 30
	}

	if v := getEnv("AI_MAX_RETRIES", ""); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			cfg.AIMaxRetries = n
		}
	}
	if cfg.AIMaxRetries == 0 {
		cfg.AIMaxRetries = 2
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}
