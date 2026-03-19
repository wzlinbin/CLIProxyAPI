package p2p

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type P2PStore struct {
	pool *pgxpool.Pool
}

func NewP2PStore(ctx context.Context, dsn string) (*P2PStore, error) {
	pool, err := pgxpool.New(ctx, strings.TrimSpace(dsn))
	if err != nil {
		return nil, fmt.Errorf("failed to create p2p connection pool: %w", err)
	}

	store := &P2PStore{pool: pool}
	if err := store.migrate(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to migrate p2p schema: %w", err)
	}

	return store, nil
}

func (s *P2PStore) Close() {
	if s == nil || s.pool == nil {
		return
	}
	s.pool.Close()
}

func HashAPIKey(apiKey string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(apiKey)))
	return hex.EncodeToString(sum[:])
}

func (s *P2PStore) migrate(ctx context.Context) error {
	queries := []string{
		`CREATE EXTENSION IF NOT EXISTS pgcrypto`,
		`CREATE TABLE IF NOT EXISTS p2p_users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			email VARCHAR(255),
			api_key VARCHAR(128) UNIQUE NOT NULL,
			key_hash VARCHAR(128) NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'active',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			last_active TIMESTAMPTZ
		)`,
		`CREATE TABLE IF NOT EXISTS p2p_providers (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES p2p_users(id) ON DELETE CASCADE,
			provider_type VARCHAR(20) NOT NULL,
			name VARCHAR(100) NOT NULL,
			base_url VARCHAR(500) NOT NULL,
			api_key TEXT NOT NULL,
			models JSONB NOT NULL DEFAULT '[]',
			daily_token_limit BIGINT NOT NULL DEFAULT 0,
			status VARCHAR(20) NOT NULL DEFAULT 'pending',
			verified_at TIMESTAMPTZ,
			last_error TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS p2p_usage_records (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES p2p_users(id) ON DELETE CASCADE,
			provider_user_id UUID REFERENCES p2p_users(id) ON DELETE SET NULL,
			provider_id UUID REFERENCES p2p_providers(id) ON DELETE SET NULL,
			model VARCHAR(255) NOT NULL,
			request_tokens BIGINT NOT NULL DEFAULT 0,
			response_tokens BIGINT NOT NULL DEFAULT 0,
			total_tokens BIGINT NOT NULL DEFAULT 0,
			success BOOLEAN NOT NULL DEFAULT TRUE,
			error_message TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS p2p_daily_stats (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES p2p_users(id) ON DELETE CASCADE,
			date DATE NOT NULL,
			contributed_tokens BIGINT NOT NULL DEFAULT 0,
			consumed_tokens BIGINT NOT NULL DEFAULT 0,
			request_count BIGINT NOT NULL DEFAULT 0,
			contributed_count BIGINT NOT NULL DEFAULT 0,
			ratio DOUBLE PRECISION NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE(user_id, date)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_p2p_users_api_key ON p2p_users(api_key)`,
		`CREATE INDEX IF NOT EXISTS idx_p2p_providers_user_id ON p2p_providers(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_p2p_providers_status ON p2p_providers(status)`,
		`CREATE INDEX IF NOT EXISTS idx_p2p_usage_records_user_id ON p2p_usage_records(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_p2p_daily_stats_user_date ON p2p_daily_stats(user_id, date)`,
	}

	for _, query := range queries {
		if _, err := s.pool.Exec(ctx, query); err != nil {
			return err
		}
	}

	return nil
}

func (s *P2PStore) CreateUser(ctx context.Context, email, apiKey string) (*User, error) {
	user := &User{
		ID:        uuid.NewString(),
		Email:     strings.TrimSpace(email),
		APIKey:    strings.TrimSpace(apiKey),
		KeyHash:   HashAPIKey(apiKey),
		Status:    UserStatusActive,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	_, err := s.pool.Exec(
		ctx,
		`INSERT INTO p2p_users (id, email, api_key, key_hash, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		user.ID, user.Email, user.APIKey, user.KeyHash, user.Status, user.CreatedAt, user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create p2p user: %w", err)
	}

	return user, nil
}

func (s *P2PStore) GetUserByAPIKey(ctx context.Context, apiKey string) (*User, error) {
	user := &User{}
	var lastActive sql.NullTime
	err := s.pool.QueryRow(
		ctx,
		`SELECT id, email, api_key, key_hash, status, created_at, updated_at, last_active
		 FROM p2p_users
		 WHERE api_key = $1`,
		strings.TrimSpace(apiKey),
	).Scan(&user.ID, &user.Email, &user.APIKey, &user.KeyHash, &user.Status, &user.CreatedAt, &user.UpdatedAt, &lastActive)
	if err != nil {
		return nil, err
	}
	if lastActive.Valid {
		user.LastActive = &lastActive.Time
	}
	return user, nil
}

func (s *P2PStore) GetUserByID(ctx context.Context, id string) (*User, error) {
	user := &User{}
	var lastActive sql.NullTime
	err := s.pool.QueryRow(
		ctx,
		`SELECT id, email, api_key, key_hash, status, created_at, updated_at, last_active
		 FROM p2p_users
		 WHERE id = $1`,
		strings.TrimSpace(id),
	).Scan(&user.ID, &user.Email, &user.APIKey, &user.KeyHash, &user.Status, &user.CreatedAt, &user.UpdatedAt, &lastActive)
	if err != nil {
		return nil, err
	}
	if lastActive.Valid {
		user.LastActive = &lastActive.Time
	}
	return user, nil
}

func (s *P2PStore) TouchUserLastActive(ctx context.Context, userID string) error {
	_, err := s.pool.Exec(
		ctx,
		`UPDATE p2p_users SET last_active = NOW(), updated_at = NOW() WHERE id = $1`,
		strings.TrimSpace(userID),
	)
	return err
}

func (s *P2PStore) UpdateUserStatus(ctx context.Context, userID string, status UserStatus) error {
	_, err := s.pool.Exec(
		ctx,
		`UPDATE p2p_users SET status = $1, updated_at = NOW() WHERE id = $2`,
		status, strings.TrimSpace(userID),
	)
	return err
}

func (s *P2PStore) CreateProvider(ctx context.Context, provider *UserProvider) error {
	if provider == nil {
		return fmt.Errorf("p2p provider is nil")
	}
	modelsJSON, err := json.Marshal(normalizeModelIDs(provider.Models))
	if err != nil {
		return fmt.Errorf("failed to encode provider models: %w", err)
	}

	now := time.Now().UTC()
	if provider.ID == "" {
		provider.ID = uuid.NewString()
	}
	if provider.CreatedAt.IsZero() {
		provider.CreatedAt = now
	}
	provider.UpdatedAt = now

	_, err = s.pool.Exec(
		ctx,
		`INSERT INTO p2p_providers (
			id, user_id, provider_type, name, base_url, api_key, models,
			daily_token_limit, status, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		provider.ID,
		provider.UserID,
		provider.ProviderType,
		provider.Name,
		normalizeBaseURL(provider.ProviderType, provider.BaseURL),
		strings.TrimSpace(provider.APIKey),
		modelsJSON,
		provider.DailyTokenLimit,
		provider.Status,
		provider.CreatedAt,
		provider.UpdatedAt,
	)
	return err
}

func (s *P2PStore) GetProvidersByUserID(ctx context.Context, userID string) ([]UserProvider, error) {
	rows, err := s.pool.Query(
		ctx,
		`SELECT id, user_id, provider_type, name, base_url, api_key, models, daily_token_limit,
		        status, verified_at, last_error, created_at, updated_at
		 FROM p2p_providers
		 WHERE user_id = $1
		 ORDER BY created_at DESC`,
		strings.TrimSpace(userID),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return collectProviders(rows)
}

func (s *P2PStore) GetVerifiedProviders(ctx context.Context) ([]UserProvider, error) {
	rows, err := s.pool.Query(
		ctx,
		`SELECT id, user_id, provider_type, name, base_url, api_key, models, daily_token_limit,
		        status, verified_at, last_error, created_at, updated_at
		 FROM p2p_providers
		 WHERE status = $1
		 ORDER BY verified_at DESC NULLS LAST, created_at DESC`,
		ProviderStatusVerified,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return collectProviders(rows)
}

func (s *P2PStore) UpdateProviderVerification(ctx context.Context, providerID string, status ProviderStatus, models []string, lastError string) error {
	modelsJSON, err := json.Marshal(normalizeModelIDs(models))
	if err != nil {
		return fmt.Errorf("failed to encode verified models: %w", err)
	}

	var verifiedAt any
	if status == ProviderStatusVerified {
		verifiedAt = time.Now().UTC()
	}

	_, err = s.pool.Exec(
		ctx,
		`UPDATE p2p_providers
		 SET status = $1,
		     models = $2,
		     verified_at = $3,
		     last_error = $4,
		     updated_at = NOW()
		 WHERE id = $5`,
		status,
		modelsJSON,
		verifiedAt,
		strings.TrimSpace(lastError),
		strings.TrimSpace(providerID),
	)
	return err
}

func (s *P2PStore) RecordUsageTransfer(ctx context.Context, record *UsageRecord) error {
	if record == nil {
		return fmt.Errorf("usage record is nil")
	}
	if record.ID == "" {
		record.ID = uuid.NewString()
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	if _, err = tx.Exec(
		ctx,
		`INSERT INTO p2p_usage_records (
			id, user_id, provider_user_id, provider_id, model,
			request_tokens, response_tokens, total_tokens, success, error_message, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		record.ID,
		record.UserID,
		nullUUID(record.ProviderUserID),
		nullUUID(record.ProviderID),
		record.Model,
		record.RequestTokens,
		record.ResponseTokens,
		record.TotalTokens,
		record.Success,
		strings.TrimSpace(record.ErrorMessage),
		record.CreatedAt,
	); err != nil {
		return err
	}

	if err = upsertDailyStats(
		ctx,
		tx,
		record.UserID,
		record.CreatedAt,
		0,
		record.TotalTokens,
		1,
		0,
	); err != nil {
		return err
	}

	if strings.TrimSpace(record.ProviderUserID) != "" {
		if err = upsertDailyStats(
			ctx,
			tx,
			record.ProviderUserID,
			record.CreatedAt,
			record.TotalTokens,
			0,
			0,
			1,
		); err != nil {
			return err
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

func (s *P2PStore) GetUserStatsSummary(ctx context.Context, userID string) (*UserStatsSummary, error) {
	summary := &UserStatsSummary{UserID: strings.TrimSpace(userID)}

	if err := s.pool.QueryRow(
		ctx,
		`SELECT
			COALESCE(SUM(contributed_tokens), 0),
			COALESCE(SUM(consumed_tokens), 0)
		 FROM p2p_daily_stats
		 WHERE user_id = $1`,
		summary.UserID,
	).Scan(&summary.TotalContributed, &summary.TotalConsumed); err != nil && err != pgx.ErrNoRows {
		return nil, err
	}

	today := time.Now().UTC().Format("2006-01-02")
	if err := s.pool.QueryRow(
		ctx,
		`SELECT
			COALESCE(contributed_tokens, 0),
			COALESCE(consumed_tokens, 0)
		 FROM p2p_daily_stats
		 WHERE user_id = $1 AND date = $2`,
		summary.UserID,
		today,
	).Scan(&summary.TodayContributed, &summary.TodayConsumed); err != nil && err != pgx.ErrNoRows {
		return nil, err
	}

	if summary.TotalContributed > 0 {
		summary.Ratio = float64(summary.TotalConsumed) / float64(summary.TotalContributed)
	}

	if err := s.pool.QueryRow(
		ctx,
		`SELECT COUNT(*), COUNT(*) FILTER (WHERE status = 'verified')
		 FROM p2p_providers
		 WHERE user_id = $1`,
		summary.UserID,
	).Scan(&summary.ProviderCount, &summary.ActiveProviderCount); err != nil {
		return nil, err
	}

	var status UserStatus
	if err := s.pool.QueryRow(
		ctx,
		`SELECT status FROM p2p_users WHERE id = $1`,
		summary.UserID,
	).Scan(&status); err != nil {
		return nil, err
	}
	summary.IsActive = status == UserStatusActive

	return summary, nil
}

func (s *P2PStore) CheckAndSuspendOverLimitUsers(ctx context.Context, maxRatio float64) (int, error) {
	if maxRatio <= 0 {
		maxRatio = 1.2
	}

	rows, err := s.pool.Query(
		ctx,
		`SELECT user_id
		 FROM p2p_daily_stats
		 GROUP BY user_id
		 HAVING SUM(contributed_tokens) > 0
		    AND SUM(consumed_tokens) > SUM(contributed_tokens) * $1`,
		maxRatio,
	)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var suspended int
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return suspended, err
		}
		if err := s.UpdateUserStatus(ctx, userID, UserStatusSuspended); err != nil {
			return suspended, err
		}
		suspended++
	}
	return suspended, rows.Err()
}

func (s *P2PStore) GetPlatformOverview(ctx context.Context) (*PlatformOverview, error) {
	overview := &PlatformOverview{UpdatedAt: time.Now().UTC()}

	if err := s.pool.QueryRow(
		ctx,
		`SELECT
			COUNT(*) AS total_users,
			COUNT(*) FILTER (WHERE status = 'active') AS active_users,
			COUNT(*) FILTER (WHERE status = 'suspended') AS suspended_users
		 FROM p2p_users`,
	).Scan(&overview.TotalUsers, &overview.ActiveUsers, &overview.SuspendedUsers); err != nil {
		return nil, err
	}

	if err := s.pool.QueryRow(
		ctx,
		`SELECT
			COUNT(*) AS total_providers,
			COUNT(*) FILTER (WHERE status = 'verified') AS verified_providers
		 FROM p2p_providers`,
	).Scan(&overview.TotalProviders, &overview.VerifiedProviders); err != nil {
		return nil, err
	}

	today := time.Now().UTC().Format("2006-01-02")
	if err := s.pool.QueryRow(
		ctx,
		`SELECT
			COALESCE(SUM(request_count), 0),
			COALESCE(SUM(consumed_tokens), 0)
		 FROM p2p_daily_stats
		 WHERE date = $1`,
		today,
	).Scan(&overview.TodayRequests, &overview.TodayTokens); err != nil {
		return nil, err
	}

	providers, err := s.GetVerifiedProviders(ctx)
	if err != nil {
		return nil, err
	}
	overview.AvailableModels = len(uniqueModelsFromProviders(providers))

	return overview, nil
}

func collectProviders(rows pgx.Rows) ([]UserProvider, error) {
	providers := make([]UserProvider, 0)
	for rows.Next() {
		var (
			provider   UserProvider
			modelsJSON []byte
			verifiedAt sql.NullTime
			lastError  sql.NullString
		)

		if err := rows.Scan(
			&provider.ID,
			&provider.UserID,
			&provider.ProviderType,
			&provider.Name,
			&provider.BaseURL,
			&provider.APIKey,
			&modelsJSON,
			&provider.DailyTokenLimit,
			&provider.Status,
			&verifiedAt,
			&lastError,
			&provider.CreatedAt,
			&provider.UpdatedAt,
		); err != nil {
			return nil, err
		}

		_ = json.Unmarshal(modelsJSON, &provider.Models)
		provider.Models = normalizeModelIDs(provider.Models)
		if verifiedAt.Valid {
			provider.VerifiedAt = &verifiedAt.Time
		}
		if lastError.Valid {
			provider.LastError = lastError.String
		}

		providers = append(providers, provider)
	}
	return providers, rows.Err()
}

func normalizeModelIDs(models []string) []string {
	if len(models) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(models))
	normalized := make([]string, 0, len(models))
	for _, model := range models {
		trimmed := strings.TrimSpace(model)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	sort.Strings(normalized)
	return normalized
}

func uniqueModelsFromProviders(providers []UserProvider) []string {
	seen := make(map[string]struct{})
	models := make([]string, 0)
	for _, provider := range providers {
		for _, model := range provider.Models {
			if model == "" {
				continue
			}
			if _, exists := seen[model]; exists {
				continue
			}
			seen[model] = struct{}{}
			models = append(models, model)
		}
	}
	sort.Strings(models)
	return models
}

func normalizeBaseURL(providerType ProviderType, baseURL string) string {
	trimmed := strings.TrimSpace(baseURL)
	if trimmed != "" {
		return strings.TrimRight(trimmed, "/")
	}
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
		return "https://dashscope.aliyuncs.com/compatible-mode/v1"
	default:
		return ""
	}
}

func nullUUID(value string) any {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return trimmed
}

func upsertDailyStats(
	ctx context.Context,
	tx pgx.Tx,
	userID string,
	at time.Time,
	contributedDelta int64,
	consumedDelta int64,
	requestDelta int64,
	contributedCountDelta int64,
) error {
	if strings.TrimSpace(userID) == "" {
		return nil
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	day := at.UTC().Format("2006-01-02")
	_, err := tx.Exec(
		ctx,
		`INSERT INTO p2p_daily_stats (
			id, user_id, date, contributed_tokens, consumed_tokens,
			request_count, contributed_count, ratio, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, $2, $3, $4, $5, $6,
			CASE WHEN $3 = 0 THEN 0 ELSE $4::DOUBLE PRECISION / $3 END,
			NOW(), NOW()
		)
		ON CONFLICT (user_id, date) DO UPDATE SET
			contributed_tokens = p2p_daily_stats.contributed_tokens + EXCLUDED.contributed_tokens,
			consumed_tokens = p2p_daily_stats.consumed_tokens + EXCLUDED.consumed_tokens,
			request_count = p2p_daily_stats.request_count + EXCLUDED.request_count,
			contributed_count = p2p_daily_stats.contributed_count + EXCLUDED.contributed_count,
			ratio = CASE
				WHEN (p2p_daily_stats.contributed_tokens + EXCLUDED.contributed_tokens) = 0 THEN 0
				ELSE (p2p_daily_stats.consumed_tokens + EXCLUDED.consumed_tokens)::DOUBLE PRECISION /
				     (p2p_daily_stats.contributed_tokens + EXCLUDED.contributed_tokens)
			END,
			updated_at = NOW()`,
		strings.TrimSpace(userID),
		day,
		contributedDelta,
		consumedDelta,
		requestDelta,
		contributedCountDelta,
	)
	return err
}
