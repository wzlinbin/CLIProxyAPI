好的，我来提供完整的代码文件，你手动创建。

📁 文件 1: internal/p2p/models.go
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
📁 文件 2: internal/p2p/store.go
package p2p

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	log "github.com/sirupsen/logrus"
)

type P2PStore struct {
	pool *pgxpool.Pool
}

func NewP2PStore(ctx context.Context, dsn string) (*P2PStore, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	store := &P2PStore{pool: pool}
	if err := store.migrate(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to migrate: %w", err)
	}

	return store, nil
}

func (s *P2PStore) Close() {
	s.pool.Close()
}

func (s *P2PStore) migrate(ctx context.Context) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS p2p_users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			email VARCHAR(255),
			api_key VARCHAR(64) UNIQUE NOT NULL,
			key_hash VARCHAR(128) NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'active',
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			last_active TIMESTAMP WITH TIME ZONE
		)`,
		`CREATE TABLE IF NOT EXISTS p2p_providers (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES p2p_users(id) ON DELETE CASCADE,
			provider_type VARCHAR(20) NOT NULL,
			name VARCHAR(100) NOT NULL,
			base_url VARCHAR(500) NOT NULL,
			api_key TEXT NOT NULL,
			models JSONB DEFAULT '[]',
			daily_token_limit BIGINT DEFAULT 0,
			status VARCHAR(20) NOT NULL DEFAULT 'pending',
			verified_at TIMESTAMP WITH TIME ZONE,
			last_error TEXT,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS p2p_usage_records (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES p2p_users(id) ON DELETE CASCADE,
			provider_user_id UUID REFERENCES p2p_users(id) ON DELETE SET NULL,
			provider_id UUID REFERENCES p2p_providers(id) ON DELETE SET NULL,
			model VARCHAR(100) NOT NULL,
			request_tokens BIGINT DEFAULT 0,
			response_tokens BIGINT DEFAULT 0,
			total_tokens BIGINT DEFAULT 0,
			success BOOLEAN DEFAULT true,
			error_message TEXT,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS p2p_daily_stats (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES p2p_users(id) ON DELETE CASCADE,
			date DATE NOT NULL,
			contributed_tokens BIGINT DEFAULT 0,
			consumed_tokens BIGINT DEFAULT 0,
			request_count BIGINT DEFAULT 0,
			contributed_count BIGINT DEFAULT 0,
			ratio DECIMAL(10,4) DEFAULT 0,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			UNIQUE(user_id, date)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_p2p_users_api_key ON p2p_users(api_key)`,
		`CREATE INDEX IF NOT EXISTS idx_p2p_providers_user_id ON p2p_providers(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_p2p_providers_status ON p2p_providers(status)`,
		`CREATE INDEX IF NOT EXISTS idx_p2p_usage_records_user_id ON p2p_usage_records(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_p2p_daily_stats_user_date ON p2p_daily_stats(user_id, date)`,
	}

	for _, q := range queries {
		if _, err := s.pool.Exec(ctx, q); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	return nil
}

func (s *P2PStore) CreateUser(ctx context.Context, email, apiKey, keyHash string) (*User, error) {
	user := &User{
		ID:        uuid.New().String(),
		Email:     email,
		APIKey:    apiKey,
		KeyHash:   keyHash,
		Status:    UserStatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := s.pool.QueryRow(ctx,
		`INSERT INTO p2p_users (id, email, api_key, key_hash, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, email, api_key, key_hash, status, created_at, updated_at, last_active`,
		user.ID, user.Email, user.APIKey, user.KeyHash, user.Status, user.CreatedAt, user.UpdatedAt,
	).Scan(&user.ID, &user.Email, &user.APIKey, &user.KeyHash, &user.Status, &user.CreatedAt, &user.UpdatedAt, &user.LastActive)

	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

func (s *P2PStore) GetUserByAPIKey(ctx context.Context, apiKey string) (*User, error) {
	user := &User{}
	err := s.pool.QueryRow(ctx,
		`SELECT id, email, api_key, key_hash, status, created_at, updated_at, last_active
		 FROM p2p_users WHERE api_key = $1`,
		apiKey,
	).Scan(&user.ID, &user.Email, &user.APIKey, &user.KeyHash, &user.Status, &user.CreatedAt, &user.UpdatedAt, &user.LastActive)

	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

func (s *P2PStore) GetUserByID(ctx context.Context, id string) (*User, error) {
	user := &User{}
	err := s.pool.QueryRow(ctx,
		`SELECT id, email, api_key, key_hash, status, created_at, updated_at, last_active
		 FROM p2p_users WHERE id = $1`,
		id,
	).Scan(&user.ID, &user.Email, &user.APIKey, &user.KeyHash, &user.Status, &user.CreatedAt, &user.UpdatedAt, &user.LastActive)

	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

func (s *P2PStore) UpdateUserStatus(ctx context.Context, userID string, status UserStatus) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE p2p_users SET status = $1, updated_at = NOW() WHERE id = $2`,
		status, userID,
	)
	return err
}

func (s *P2PStore) CreateProvider(ctx context.Context, provider *UserProvider) error {
	modelsJSON, _ := json.Marshal(provider.Models)

	err := s.pool.QueryRow(ctx,
		`INSERT INTO p2p_providers (user_id, provider_type, name, base_url, api_key, models, daily_token_limit, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 RETURNING id`,
		provider.UserID, provider.ProviderType, provider.Name, provider.BaseURL, provider.APIKey,
		modelsJSON, provider.DailyTokenLimit, provider.Status, provider.CreatedAt, provider.UpdatedAt,
	).Scan(&provider.ID)

	return err
}

func (s *P2PStore) GetProvidersByUserID(ctx context.Context, userID string) ([]UserProvider, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, user_id, provider_type, name, base_url, api_key, models, daily_token_limit, status, verified_at, last_error, created_at, updated_at
		 FROM p2p_providers WHERE user_id = $1`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var providers []UserProvider
	for rows.Next() {
		var p UserProvider
		var modelsJSON []byte
		var verifiedAt sql.NullTime
		var lastError sql.NullString

		err := rows.Scan(&p.ID, &p.UserID, &p.ProviderType, &p.Name, &p.BaseURL, &p.APIKey,
			&modelsJSON, &p.DailyTokenLimit, &p.Status, &verifiedAt, &lastError, &p.CreatedAt, &p.UpdatedAt)
		if err != nil {
			return nil, err
		}

		json.Unmarshal(modelsJSON, &p.Models)
		if verifiedAt.Valid {
			p.VerifiedAt = &verifiedAt.Time
		}
		if lastError.Valid {
			p.LastError = lastError.String
		}

		providers = append(providers, p)
	}

	return providers, nil
}

func (s *P2PStore) GetVerifiedProviders(ctx context.Context) ([]UserProvider, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, user_id, provider_type, name, base_url, api_key, models, daily_token_limit, status, verified_at, last_error, created_at, updated_at
		 FROM p2p_providers WHERE status = $1`,
		ProviderStatusVerified,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var providers []UserProvider
	for rows.Next() {
		var p UserProvider
		var modelsJSON []byte
		var verifiedAt sql.NullTime
		var lastError sql.NullString

		err := rows.Scan(&p.ID, &p.UserID, &p.ProviderType, &p.Name, &p.BaseURL, &p.APIKey,
			&modelsJSON, &p.DailyTokenLimit, &p.Status, &verifiedAt, &lastError, &p.CreatedAt, &p.UpdatedAt)
		if err != nil {
			return nil, err
		}

		json.Unmarshal(modelsJSON, &p.Models)
		if verifiedAt.Valid {
			p.VerifiedAt = &verifiedAt.Time
		}
		if lastError.Valid {
			p.LastError = lastError.String
		}

		providers = append(providers, p)
	}

	return providers, nil
}

func (s *P2PStore) UpdateProviderStatus(ctx context.Context, providerID string, status ProviderStatus, lastError string) error {
	var verifiedAt interface{}
	if status == ProviderStatusVerified {
		verifiedAt = time.Now()
	}
	_, err := s.pool.Exec(ctx,
		`UPDATE p2p_providers SET status = $1, last_error = $2, verified_at = $3, updated_at = NOW() WHERE id = $4`,
		status, lastError, verifiedAt, providerID,
	)
	return err
}

func (s *P2PStore) RecordUsage(ctx context.Context, record *UsageRecord) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO p2p_usage_records (user_id, provider_user_id, provider_id, model, request_tokens, response_tokens, total_tokens, success, error_message, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		record.UserID, record.ProviderUserID, record.ProviderID, record.Model,
		record.RequestTokens, record.ResponseTokens, record.TotalTokens, record.Success, record.ErrorMessage, record.CreatedAt,
	)
	return err
}

func (s *P2PStore) UpdateDailyStats(ctx context.Context, userID string, contributedDelta, consumedDelta int64) error {
	now := time.Now()
	today := now.Format("2006-01-02")

	_, err := s.pool.Exec(ctx,
		`INSERT INTO p2p_daily_stats (user_id, date, contributed_tokens, consumed_tokens, ratio, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, CASE WHEN $3 = 0 THEN 0 ELSE $4::DECIMAL / $3 END, $5, $5)
		 ON CONFLICT (user_id, date) DO UPDATE SET
			contributed_tokens = p2p_daily_stats.contributed_tokens + $3,
			consumed_tokens = p2p_daily_stats.consumed_tokens + $4,
			ratio = CASE WHEN p2p_daily_stats.contributed_tokens + $3 = 0 THEN 0 
				ELSE (p2p_daily_stats.consumed_tokens + $4)::DECIMAL / (p2p_daily_stats.contributed_tokens + $3) END,
			updated_at = $5`,
		userID, today, contributedDelta, consumedDelta, now,
	)

	return err
}

func (s *P2PStore) GetUserStatsSummary(ctx context.Context, userID string) (*UserStatsSummary, error) {
	summary := &UserStatsSummary{UserID: userID}

	// Get total stats
	err := s.pool.QueryRow(ctx,
		`SELECT 
			COALESCE(SUM(contributed_tokens), 0) as total_contributed,
			COALESCE(SUM(consumed_tokens), 0) as total_consumed
		 FROM p2p_daily_stats WHERE user_id = $1`,
		userID,
	).Scan(&summary.TotalContributed, &summary.TotalConsumed)

	if err != nil && err != pgx.ErrNoRows {
		return nil, err
	}

	// Get today stats
	today := time.Now().Format("2006-01-02")
	err = s.pool.QueryRow(ctx,
		`SELECT 
			COALESCE(contributed_tokens, 0) as today_contributed,
			COALESCE(consumed_tokens, 0) as today_consumed
		 FROM p2p_daily_stats WHERE user_id = $1 AND date = $2`,
		userID, today,
	).Scan(&summary.TodayContributed, &summary.TodayConsumed)

	if err != nil && err != pgx.ErrNoRows {
		return nil, err
	}

	// Calculate ratio
	if summary.TotalContributed > 0 {
		summary.Ratio = float64(summary.TotalConsumed) / float64(summary.TotalContributed)
	}

	// Get provider counts
	err = s.pool.QueryRow(ctx,
		`SELECT COUNT(*) as total, COUNT(*) FILTER (WHERE status = 'verified') as active
		 FROM p2p_providers WHERE user_id = $1`,
		userID,
	).Scan(&summary.ProviderCount, &summary.ActiveProviderCount)

	if err != nil {
		return nil, err
	}

	// Check if user is active
	var status UserStatus
	err = s.pool.QueryRow(ctx, `SELECT status FROM p2p_users WHERE id = $1`, userID).Scan(&status)
	if err != nil {
		return nil, err
	}
	summary.IsActive = status == UserStatusActive

	return summary, nil
}

func (s *P2PStore) CheckAndSuspendOverLimitUsers(ctx context.Context) (int, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT user_id, SUM(consumed_tokens) as consumed, SUM(contributed_tokens) as contributed
		 FROM p2p_daily_stats
		 GROUP BY user_id
		 HAVING SUM(contributed_tokens) > 0 AND SUM(consumed_tokens) > SUM(contributed_tokens) * 1.2`,
	)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var suspendedCount int
	for rows.Next() {
		var userID string
		var consumed, contributed int64
		if err := rows.Scan(&userID, &consumed, &contributed); err != nil {
			log.WithError(err).Error("Failed to scan over-limit user")
			continue
		}

		if err := s.UpdateUserStatus(ctx, userID, UserStatusSuspended); err != nil {
			log.WithError(err).WithField("user_id", userID).Error("Failed to suspend over-limit user")
			continue
		}

		log.WithFields(log.Fields{
			"user_id":     userID,
			"consumed":    consumed,
			"contributed": contributed,
			"ratio":       float64(consumed) / float64(contributed),
		}).Info("Suspended over-limit user")

		suspendedCount++
	}

	return suspendedCount, nil
}
📁 文件 3: internal/p2p/verifier.go
package p2p

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

type Verifier struct {
	client *http.Client
}

func NewVerifier() *Verifier {
	return &Verifier{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (v *Verifier) VerifyProvider(ctx context.Context, provider *UserProvider) *VerificationResult {
	result := &VerificationResult{}

	baseURL := provider.BaseURL
	if baseURL == "" {
		baseURL = v.getDefaultBaseURL(provider.ProviderType)
	}

	// Step 1: List models
	models, err := v.listModels(ctx, provider.ProviderType, baseURL, provider.APIKey)
	if err != nil {
		result.Success = false
		result.ErrorMessage = fmt.Sprintf("Failed to list models: %v", err)
		return result
	}
	result.Models = models

	// Step 2: Test request
	testPassed, err := v.testRequest(ctx, provider.ProviderType, baseURL, provider.APIKey, models)
	if err != nil {
		result.Success = false
		result.ErrorMessage = fmt.Sprintf("Test request failed: %v", err)
		return result
	}
	result.TestPassed = testPassed

	result.Success = true
	return result
}

func (v *Verifier) getDefaultBaseURL(providerType ProviderType) string {
	switch providerType {
	case ProviderTypeOpenAI:
		return "https://api.openai.com/v1"
	case ProviderTypeClaude:
		return "https://api.anthropic.com/v1"
	case ProviderTypeGemini:
		return "https://generativelanguage.googleapis.com/v1beta"
	case ProviderTypeCodex:
		return "https://api.openai.com/v1"
	case ProviderTypeQwen:
		return "https://dashscope.aliyuncs.com/api/v1"
	default:
		return ""
	}
}

func (v *Verifier) listModels(ctx context.Context, providerType ProviderType, baseURL, apiKey string) ([]string, error) {
	var modelsURL string
	var headers map[string]string

	switch providerType {
	case ProviderTypeOpenAI, ProviderTypeCodex:
		modelsURL = baseURL + "/models"
		headers = map[string]string{"Authorization": "Bearer " + apiKey}
	case ProviderTypeClaude:
		return []string{"claude-3-5-sonnet-20241022", "claude-3-opus-20240229", "claude-3-haiku-20240307"}, nil
	case ProviderTypeGemini:
		modelsURL = baseURL + "/models?key=" + apiKey
		headers = map[string]string{}
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", providerType)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", modelsURL, nil)
	if err != nil {
		return nil, err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := v.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return v.parseModelsResponse(providerType, body)
}

func (v *Verifier) parseModelsResponse(providerType ProviderType, body []byte) ([]string, error) {
	var models []string

	switch providerType {
	case ProviderTypeOpenAI, ProviderTypeCodex:
		var resp struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, err
		}
		for _, m := range resp.Data {
			models = append(models, m.ID)
		}
	case ProviderTypeGemini:
		var resp struct {
			Models []struct {
				Name string `json:"name"`
			} `json:"models"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, err
		}
		for _, m := range resp.Models {
			name := strings.TrimPrefix(m.Name, "models/")
			models = append(models, name)
		}
	}

	return models, nil
}

func (v *Verifier) testRequest(ctx context.Context, providerType ProviderType, baseURL, apiKey string, models []string) (bool, error) {
	if len(models) == 0 {
		return false, fmt.Errorf("no models available for testing")
	}

	testModel := v.selectTestModel(providerType, models)

	switch providerType {
	case ProviderTypeOpenAI, ProviderTypeCodex:
		return v.testOpenAI(ctx, baseURL, apiKey, testModel)
	case ProviderTypeClaude:
		return v.testClaude(ctx, baseURL, apiKey, testModel)
	case ProviderTypeGemini:
		return v.testGemini(ctx, baseURL, apiKey, testModel)
	default:
		return false, fmt.Errorf("unsupported provider type: %s", providerType)
	}
}

func (v *Verifier) selectTestModel(providerType ProviderType, models []string) string {
	preferred := []string{
		"gpt-4o-mini", "gpt-3.5-turbo", "gpt-4o",
		"claude-3-5-haiku-20241022", "claude-3-haiku-20240307",
		"gemini-2.0-flash", "gemini-1.5-flash", "gemini-pro",
	}

	for _, p := range preferred {
		for _, m := range models {
			if strings.Contains(m, p) || m == p {
				return m
			}
		}
	}

	return models[0]
}

func (v *Verifier) testOpenAI(ctx context.Context, baseURL, apiKey, model string) (bool, error) {
	reqBody := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": "Say 'test'"},
		},
		"max_tokens": 5,
	}

	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return false, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := v.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

func (v *Verifier) testClaude(ctx context.Context, baseURL, apiKey, model string) (bool, error) {
	reqBody := map[string]interface{}{
		"model":      model,
		"max_tokens": 5,
		"messages": []map[string]string{
			{"role": "user", "content": "Say 'test'"},
		},
	}

	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return false, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := v.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

func (v *Verifier) testGemini(ctx context.Context, baseURL, apiKey, model string) (bool, error) {
	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"parts": []map[string]string{{"text": "Say test"}}},
		},
	}

	body, _ := json.Marshal(reqBody)

	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", baseURL, model, apiKey)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return false, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := v.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}
📁 文件 4: internal/p2p/handlers.go
package p2p

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

type Handlers struct {
	store    *P2PStore
	verifier *Verifier
}

func NewHandlers(store *P2PStore) *Handlers {
	return &Handlers{
		store:    store,
		verifier: NewVerifier(),
	}
}

func (h *Handlers) RegisterRoutes(r *gin.Engine) {
	p2p := r.Group("/p2p")
	{
		p2p.GET("", h.ServeFrontend)
		p2p.GET("/", h.ServeFrontend)
		p2p.POST("/register", h.Register)
		p2p.GET("/info", h.GetInfo)
		p2p.GET("/stats", h.GetStats)
		p2p.GET("/models", h.GetAvailableModels)
	}
}

func (h *Handlers) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.APIKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "API key is required"})
		return
	}
	if req.ProviderType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Provider type is required"})
		return
	}

	// Generate user API key
	userAPIKey := "sk-p2p-" + uuid.New().String()[:24]
	keyHash := sha256.Sum256([]byte(userAPIKey))
	keyHashStr := hex.EncodeToString(keyHash[:])

	// Create user
	ctx := c.Request.Context()
	user, err := h.store.CreateUser(ctx, req.Email, userAPIKey, keyHashStr)
	if err != nil {
		log.WithError(err).Error("Failed to create user")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	// Create provider
	provider := &UserProvider{
		ID:              uuid.New().String(),
		UserID:          user.ID,
		ProviderType:    req.ProviderType,
		Name:            req.Name,
		BaseURL:         req.BaseURL,
		APIKey:          req.APIKey,
		Models:          req.Models,
		DailyTokenLimit: req.DailyTokenLimit,
		Status:          ProviderStatusPending,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	if err := h.store.CreateProvider(ctx, provider); err != nil {
		log.WithError(err).Error("Failed to create provider")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create provider"})
		return
	}

	// Verify provider in background
	go func() {
		bgCtx := c.Request.Context()
		result := h.verifier.VerifyProvider(bgCtx, provider)

		var status ProviderStatus
		var lastError string
		if result.Success {
			status = ProviderStatusVerified
			provider.Models = result.Models
		} else {
			status = ProviderStatusFailed
			lastError = result.ErrorMessage
		}

		if err := h.store.UpdateProviderStatus(bgCtx, provider.ID, status, lastError); err != nil {
			log.WithError(err).Error("Failed to update provider status")
		}
	}()

	c.JSON(http.StatusOK, RegisterResponse{
		Success:              true,
		Message:              "Registration successful. Your provider is being verified.",
		UserID:               user.ID,
		APIKey:               userAPIKey,
		RequiresVerification: true,
	})
}

func (h *Handlers) GetInfo(c *gin.Context) {
	apiKey := c.GetHeader("X-API-Key")
	if apiKey == "" {
		apiKey = c.Query("api_key")
	}

	if apiKey == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "API key required"})
		return
	}

	ctx := c.Request.Context()
	user, err := h.store.GetUserByAPIKey(ctx, apiKey)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
		return
	}

	providers, err := h.store.GetProvidersByUserID(ctx, user.ID)
	if err != nil {
		log.WithError(err).Error("Failed to get providers")
		providers = nil
	}

	stats, err := h.store.GetUserStatsSummary(ctx, user.ID)
	if err != nil {
		log.WithError(err).Error("Failed to get stats")
		stats = &UserStatsSummary{}
	}

	c.JSON(http.StatusOK, UserInfo{
		ID:        user.ID,
		Email:     user.Email,
		APIKey:    user.APIKey,
		Status:    user.Status,
		CreatedAt: user.CreatedAt,
		Providers: providers,
		Stats:     *stats,
	})
}

func (h *Handlers) GetStats(c *gin.Context) {
	apiKey := c.GetHeader("X-API-Key")
	if apiKey == "" {
		apiKey = c.Query("api_key")
	}

	if apiKey == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "API key required"})
		return
	}

	ctx := c.Request.Context()
	user, err := h.store.GetUserByAPIKey(ctx, apiKey)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
		return
	}

	stats, err := h.store.GetUserStatsSummary(ctx, user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get stats"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

func (h *Handlers) GetAvailableModels(c *gin.Context) {
	ctx := c.Request.Context()
	providers, err := h.store.GetVerifiedProviders(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get providers"})
		return
	}

	modelSet := make(map[string]bool)
	for _, p := range providers {
		for _, m := range p.Models {
			modelSet[m] = true
		}
	}

	var models []string
	for m := range modelSet {
		models = append(models, m)
	}

	c.JSON(http.StatusOK, gin.H{"models": models})
}

func (h *Handlers) ServeFrontend(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, frontendHTML)
}