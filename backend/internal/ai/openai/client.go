package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ayush/supportiq/internal/ai/parser"
	"github.com/ayush/supportiq/internal/ai/prompt"
	"github.com/ayush/supportiq/internal/ai/provider"
	replyparser "github.com/ayush/supportiq/internal/ai/reply/parser"
	replyprompt "github.com/ayush/supportiq/internal/ai/reply/prompt"
	replyprovider "github.com/ayush/supportiq/internal/ai/reply/provider"
	replyvalidator "github.com/ayush/supportiq/internal/ai/reply/validator"
	aivalidator "github.com/ayush/supportiq/internal/ai/validator"
	"github.com/ayush/supportiq/internal/utils"
)

const apiBaseURL = "https://api.openai.com/v1/chat/completions"

// Client implements provider.Provider and replyprovider.ReplyProvider using the OpenAI REST API.
type Client struct {
	apiKey           string
	model            string
	maxRetries       int
	maxReplyTokens   int
	replyTemperature float64
	httpClient       *http.Client
}

func NewClient(apiKey, model string, timeout time.Duration, maxRetries int) *Client {
	return &Client{
		apiKey:           apiKey,
		model:            model,
		maxRetries:       maxRetries,
		maxReplyTokens:   1024,
		replyTemperature: 0.3,
		httpClient:       &http.Client{Timeout: timeout},
	}
}

// NewClientWithReplyConfig creates a Client with explicit reply generation parameters.
func NewClientWithReplyConfig(apiKey, model string, timeout time.Duration, maxRetries, maxReplyTokens int, replyTemperature float64) *Client {
	c := NewClient(apiKey, model, timeout, maxRetries)
	c.maxReplyTokens = maxReplyTokens
	c.replyTemperature = replyTemperature
	return c
}

// ─── Internal request / response types ───────────────────────────────────────

type openAIRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Temperature float64         `json:"temperature"`
	MaxTokens   int             `json:"max_tokens"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

// ─── provider.Provider implementation ────────────────────────────────────────

// Analyze calls the OpenAI API with automatic retries and structured logging.
func (c *Client) Analyze(ctx context.Context, req provider.AnalysisRequest) (*provider.AnalysisResult, error) {
	promptText := prompt.BuildTicketAnalysisPrompt(
		req.Subject, req.Description, req.CustomerName, req.Category, req.Priority,
	)

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			utils.Logger.WithField("attempt", attempt).
				WithField("model", c.model).
				Info("AI: Retrying OpenAI API call")
		}

		result, err := c.callOnce(ctx, promptText)
		if err == nil {
			return result, nil
		}
		lastErr = err
		utils.Logger.WithError(err).
			WithField("attempt", attempt).
			Warn("AI: OpenAI API call failed")
	}

	return nil, fmt.Errorf("all %d attempt(s) failed, last error: %w", c.maxRetries+1, lastErr)
}

// ─── replyprovider.ReplyProvider implementation ───────────────────────────────

// GenerateReply calls the OpenAI API to generate a customer support reply using RAG context.
func (c *Client) GenerateReply(ctx context.Context, req replyprovider.ReplyRequest) (*replyprovider.ReplyResult, error) {
	promptText := replyprompt.BuildReplyPrompt(req)

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			utils.Logger.WithField("attempt", attempt).
				WithField("model", c.model).
				Info("AI Reply: Retrying OpenAI API call")
		}

		rawText, err := c.callAPI(ctx, promptText, c.replyTemperature, c.maxReplyTokens)
		if err != nil {
			lastErr = err
			utils.Logger.WithError(err).WithField("attempt", attempt).Warn("AI Reply: OpenAI API call failed")
			continue
		}

		parsed, err := replyparser.Parse(rawText)
		if err != nil {
			lastErr = err
			utils.Logger.WithError(err).Warn("AI Reply: Response parse failed")
			continue
		}

		if err := replyvalidator.Validate(parsed); err != nil {
			lastErr = err
			utils.Logger.WithError(err).Warn("AI Reply: Response validation failed")
			continue
		}

		return &replyprovider.ReplyResult{
			Reply:      parsed.Reply,
			Confidence: parsed.Confidence,
		}, nil
	}

	return nil, fmt.Errorf("all %d attempt(s) failed, last error: %w", c.maxRetries+1, lastErr)
}

// ─── Shared HTTP layer ────────────────────────────────────────────────────────

// callOnce wraps callAPI with analysis-specific parsing and validation.
func (c *Client) callOnce(ctx context.Context, promptText string) (*provider.AnalysisResult, error) {
	rawText, err := c.callAPI(ctx, promptText, 0.1, 512)
	if err != nil {
		return nil, err
	}

	parsed, err := parser.Parse(rawText)
	if err != nil {
		utils.Logger.WithError(err).Warn("AI: Response parse failed")
		return nil, err
	}

	if err := aivalidator.Validate(parsed); err != nil {
		utils.Logger.WithError(err).Warn("AI: Response validation failed")
		return nil, err
	}

	return &provider.AnalysisResult{
		Category:        parsed.Category,
		Priority:        parsed.Priority,
		Sentiment:       parsed.Sentiment,
		RecommendedTeam: parsed.RecommendedTeam,
		Confidence:      parsed.Confidence,
		Summary:         parsed.Summary,
		Tags:            parsed.Tags,
	}, nil
}

// callAPI performs a single HTTP call to the OpenAI Chat Completions API and returns the raw text response.
func (c *Client) callAPI(ctx context.Context, promptText string, temperature float64, maxTokens int) (string, error) {
	start := time.Now()

	reqBody := openAIRequest{
		Model: c.model,
		Messages: []openAIMessage{
			{Role: "user", Content: promptText},
		},
		Temperature: temperature,
		MaxTokens:   maxTokens,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request body: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBaseURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("failed to build HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	latency := time.Since(start)

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		utils.Logger.WithField("status", resp.StatusCode).
			WithField("latency_ms", latency.Milliseconds()).
			Warn("AI: OpenAI API returned non-200")
		return "", fmt.Errorf("openai API returned HTTP %d: %s", resp.StatusCode, string(respBytes))
	}

	var openAIResp openAIResponse
	if err := json.Unmarshal(respBytes, &openAIResp); err != nil {
		return "", fmt.Errorf("failed to decode OpenAI response body: %w", err)
	}

	if openAIResp.Error != nil {
		return "", fmt.Errorf("openai API error: %s", openAIResp.Error.Message)
	}

	if len(openAIResp.Choices) == 0 {
		return "", fmt.Errorf("openai returned an empty choices list")
	}

	rawText := openAIResp.Choices[0].Message.Content

	utils.Logger.WithField("latency_ms", latency.Milliseconds()).
		WithField("model", c.model).
		WithField("tokens_total", openAIResp.Usage.TotalTokens).
		WithField("tokens_prompt", openAIResp.Usage.PromptTokens).
		WithField("tokens_response", openAIResp.Usage.CompletionTokens).
		Info("AI: OpenAI response received")

	return rawText, nil
}
