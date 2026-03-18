package p2p

import (
	"time"
)

type UserStatus string

const (
	UserStatusActive    UserStatus = "active"
	UserStatusSuspended UserStatus = "suspended"
	UserStatusDisabled  UserStatus = "disabled"
)

type ProviderStatus string

const (
	ProviderStatusPending  ProviderStatus = "pending"
	ProviderStatusVerified ProviderStatus = "verified"
	ProviderStatusFailed   ProviderStatus = "failed"
)

type ProviderType string

const (
	ProviderTypeOpenAI ProviderType = "openai"
	ProviderTypeClaude ProviderType = "claude"
	ProviderTypeGemini ProviderType = "gemini"
	ProviderTypeCodex  ProviderType = "codex"
	ProviderTypeQwen   ProviderType = "qwen"
)

type User struct {
	ID         string     `json:"id" db:"id"`
	Email      string     `json:"email,omitempty" db:"email"`
	APIKey     string     `json:"api_key" db:"api_key"`
	KeyHash    string     `json:"-" db:"key_hash"`
	Status     UserStatus `json:"status" db:"status"`
	CreatedAt  time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at" db:"updated_at"`
	LastActive *time.Time `json:"last_active,omitempty" db:"last_active"`
}

type UserProvider struct {
	ID               string         `json:"id" db:"id"`
	UserID           string         `json:"user_id" db:"user_id"`
	ProviderType     ProviderType   `json:"provider_type" db:"provider_type"`
	Name             string         `json:"name" db:"name"`
	BaseURL          string         `json:"base_url" db:"base_url"`
	APIKey           string         `json:"-" db:"api_key"`
	Models           []string       `json:"models" db:"models"`
	DailyTokenLimit  int64          `json:"daily_token_limit" db:"daily_token_limit"`
	Status           ProviderStatus `json:"status" db:"status"`
	VerifiedAt       *time.Time     `json:"verified_at,omitempty" db:"verified_at"`
	LastError        string         `json:"last_error,omitempty" db:"last_error"`
	CreatedAt        time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at" db:"updated_at"`
}

type UsageRecord struct {
	ID              string    `json:"id" db:"id"`
	UserID          string    `json:"user_id" db:"user_id"`
	ProviderUserID  string    `json:"provider_user_id" db:"provider_user_id"`
	ProviderID      string    `json:"provider_id" db:"provider_id"`
	Model           string    `json:"model" db:"model"`
	RequestTokens   int64     `json:"request_tokens" db:"request_tokens"`
	ResponseTokens  int64     `json:"response_tokens" db:"response_tokens"`
	TotalTokens     int64     `json:"total_tokens" db:"total_tokens"`
	Success         bool      `json:"success" db:"success"`
	ErrorMessage    string    `json:"error_message,omitempty" db:"error_message"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
}

type DailyStats struct {
	ID                string    `json:"id" db:"id"`
	UserID            string    `json:"user_id" db:"user_id"`
	Date              time.Time `json:"date" db:"date"`
	ContributedTokens int64     `json:"contributed_tokens" db:"contributed_tokens"`
	ConsumedTokens    int64     `json:"consumed_tokens" db:"consumed_tokens"`
	RequestCount      int64     `json:"request_count" db:"request_count"`
	ContributedCount  int64     `json:"contributed_count" db:"contributed_count"`
	Ratio             float64   `json:"ratio" db:"ratio"`
	CreatedAt         time.Time `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time `json:"updated_at" db:"updated_at"`
}

type RegisterRequest struct {
	Email           string       `json:"email,omitempty"`
	ProviderType    ProviderType `json:"provider_type"`
	Name            string       `json:"name"`
	BaseURL         string       `json:"base_url"`
	APIKey          string       `json:"api_key"`
	Models          []string     `json:"models"`
	DailyTokenLimit int64        `json:"daily_token_limit"`
}

type RegisterResponse struct {
	Success              bool   `json:"success"`
	Message              string `json:"message"`
	UserID               string `json:"user_id,omitempty"`
	APIKey               string `json:"api_key,omitempty"`
	RequiresVerification bool   `json:"requires_verification"`
}

type UserInfo struct {
	ID        string           `json:"id"`
	Email     string           `json:"email,omitempty"`
	APIKey    string           `json:"api_key"`
	Status    UserStatus       `json:"status"`
	CreatedAt time.Time        `json:"created_at"`
	Providers []UserProvider   `json:"providers"`
	Stats     UserStatsSummary `json:"stats"`
}

type UserStatsSummary struct {
	UserID              string  `json:"user_id"`
	TotalContributed    int64   `json:"total_contributed"`
	TotalConsumed       int64   `json:"total_consumed"`
	TodayContributed    int64   `json:"today_contributed"`
	TodayConsumed       int64   `json:"today_consumed"`
	Ratio               float64 `json:"ratio"`
	IsActive            bool    `json:"is_active"`
	ProviderCount       int     `json:"provider_count"`
	ActiveProviderCount int     `json:"active_provider_count"`
}

type VerificationResult struct {
	Success      bool     `json:"success"`
	ErrorMessage string   `json:"error_message,omitempty"`
	Models       []string `json:"models,omitempty"`
	TestPassed   bool     `json:"test_passed"`
}