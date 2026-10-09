package adapter

import (
	"context"
	"encoding/json"
	"time"
)

type CoreParams struct {
	Temperature *float64 `json:"temperature,omitempty"`
	TopP        *float64 `json:"top_p,omitempty"`
	MaxTokens   *int     `json:"max_tokens,omitempty"`
	Stop        []string `json:"stop,omitempty"`
	Seed        *int64   `json:"seed,omitempty"`
	Stream      *bool    `json:"stream,omitempty"`
}

type Message struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type Input struct {
	System   string    `json:"system,omitempty"`
	Messages []Message `json:"messages"`
}

type Provider struct {
	ID          int64             `json:"id"`
	Name        string            `json:"name"`
	Protocol    string            `json:"protocol"` // "openai_chat" | "anthropic"
	BaseURL     string            `json:"base_url"`
	APIKeyRef   string            `json:"api_key_ref"`
	Headers     map[string]string `json:"headers"`
	StreamUsage bool              `json:"stream_usage"` // Legacy setting; request bodies no longer inject stream_options.
	CreatedAt   int64             `json:"created_at"`
	UpdatedAt   int64             `json:"updated_at"`
}

type Target struct {
	ID             int64          `json:"id"`
	ProviderID     int64          `json:"provider_id"`
	Label          string         `json:"label"`
	ModelName      string         `json:"model_name"`
	Core           CoreParams     `json:"core"`
	Extra          map[string]any `json:"extra"`
	MaxTokensField string         `json:"max_tokens_field"` // "max_tokens" | "max_completion_tokens"
	CreatedAt      int64          `json:"created_at"`
	UpdatedAt      int64          `json:"updated_at"`
}

type Event struct {
	Kind string // "content" | "reasoning"
	Text string
}

type Metrics struct {
	TTFBms         *float64 `json:"ttfb_ms"`
	FirstTokenMs   *float64 `json:"first_token_ms"`
	FirstContentMs *float64 `json:"first_content_ms"`
	TotalMs        *float64 `json:"total_ms"`
}

type Result struct {
	Status           string // done | error | canceled
	HTTPStatus       int
	Err              string
	FinishReason     string
	UsageJSON        json.RawMessage
	PromptTokens     *int
	CompletionTokens *int
	ReasoningTokens  *int
	ResponseModel    string
	ProviderReqID    string
	RespRaw          []byte
	RespTruncated    bool
	Output           string
	Reasoning        string
	Metrics          Metrics
}

type Adapter interface {
	Do(ctx context.Context, p Provider, apiKey string, body []byte, on func(Event)) *Result
}

type Clock func() time.Time
