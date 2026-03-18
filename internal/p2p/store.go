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