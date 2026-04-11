package usage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/util"
	coreusage "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/usage"
	log "github.com/sirupsen/logrus"
	_ "modernc.org/sqlite"
)

const (
	defaultUsageSQLiteFilename = "usage-statistics.sqlite"
	sqliteDriverName           = "sqlite"
	defaultRetentionDays       = 90
	defaultMaxPersistedRows    = 250000
	sqlitePruneEveryInserts    = 250
	sqliteTimestampLayout      = "2006-01-02T15:04:05.000000000Z07:00"
	sqliteWriteTimeout         = 5 * time.Second
)

var defaultSQLitePlugin = &SQLitePlugin{}

// SQLitePlugin persists usage records into a local SQLite database so statistics survive restarts.
type SQLitePlugin struct {
	mu    sync.RWMutex
	store *sqliteStore
}

type sqliteRetentionSettings struct {
	RetentionDays int
	MaxRows       int
}

type BillingModelPrice struct {
	ModelName          string  `json:"model_name"`
	InputPricePerM     float64 `json:"input_price_per_m_tokens"`
	OutputPricePerM    float64 `json:"output_price_per_m_tokens"`
	ReasoningPricePerM float64 `json:"reasoning_price_per_m_tokens"`
	CachedPricePerM    float64 `json:"cached_price_per_m_tokens"`
	UpdatedAt          string  `json:"updated_at,omitempty"`
}

// EnableSQLitePersistence opens the usage SQLite database, loads historical records,
// and enables background persistence for newly published usage events.
func EnableSQLitePersistence(ctx context.Context, configFilePath string, cfg *config.Config) error {
	path := ResolveSQLitePersistencePath(configFilePath, cfg)
	if path == "" {
		return nil
	}
	store, pruned, snapshot, err := enableSQLitePersistenceWithRecovery(ctx, path, true)
	if err != nil {
		return err
	}
	result := defaultRequestStatistics.MergeSnapshot(snapshot)
	prev := defaultSQLitePlugin.swapStore(store)
	if prev != nil {
		_ = prev.Close()
	}
	log.Infof(
		"usage sqlite persistence enabled: %s (loaded=%d skipped=%d pruned=%d retention_days=%d max_rows=%d)",
		path,
		result.Added,
		result.Skipped,
		pruned,
		store.retention.RetentionDays,
		store.retention.MaxRows,
	)
	return nil
}

func enableSQLitePersistenceWithRecovery(ctx context.Context, path string, allowRecover bool) (*sqliteStore, int64, StatisticsSnapshot, error) {
	store, pruned, snapshot, err := openSQLitePersistenceStore(ctx, path)
	if err == nil {
		return store, pruned, snapshot, nil
	}
	if !allowRecover || !isSQLiteCorruptionError(err) {
		return nil, 0, StatisticsSnapshot{}, err
	}

	if recoverErr := rotateCorruptSQLiteDatabase(path); recoverErr != nil {
		return nil, 0, StatisticsSnapshot{}, fmt.Errorf("usage sqlite: recover corrupt database: %w (original error: %v)", recoverErr, err)
	}
	log.WithError(err).Warnf("usage sqlite corruption detected at %s; moved corrupt database aside and recreating a fresh store", path)

	store, pruned, snapshot, err = openSQLitePersistenceStore(ctx, path)
	if err != nil {
		return nil, 0, StatisticsSnapshot{}, err
	}
	return store, pruned, snapshot, nil
}

func openSQLitePersistenceStore(ctx context.Context, path string) (*sqliteStore, int64, StatisticsSnapshot, error) {
	store, err := newSQLiteStore(path)
	if err != nil {
		return nil, 0, StatisticsSnapshot{}, err
	}
	pruned, err := store.Prune(ctx)
	if err != nil {
		_ = store.Close()
		return nil, 0, StatisticsSnapshot{}, err
	}
	snapshot, err := store.LoadSnapshot(ctx)
	if err != nil {
		_ = store.Close()
		return nil, 0, StatisticsSnapshot{}, err
	}
	return store, pruned, snapshot, nil
}

func isSQLiteCorruptionError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	for _, marker := range []string{
		"database disk image is malformed",
		"file is not a database",
		"malformed database schema",
		"database malformed",
		"database corrupt",
		"malformed",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

func rotateCorruptSQLiteDatabase(path string) error {
	cleaned := filepath.Clean(strings.TrimSpace(path))
	if cleaned == "" {
		return fmt.Errorf("usage sqlite: empty database path")
	}

	suffix := ".corrupt-" + time.Now().UTC().Format("20060102T150405.000000000Z")
	renamed := 0
	for _, candidate := range []string{
		cleaned,
		cleaned + "-wal",
		cleaned + "-shm",
		cleaned + "-journal",
	} {
		info, err := os.Stat(candidate)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) || os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("usage sqlite: stat corrupt database file %s: %w", candidate, err)
		}
		if info.IsDir() {
			continue
		}
		if err := os.Rename(candidate, candidate+suffix); err != nil {
			return fmt.Errorf("usage sqlite: move corrupt database file %s: %w", candidate, err)
		}
		renamed++
	}
	if renamed == 0 {
		return fmt.Errorf("usage sqlite: no database files found at %s", cleaned)
	}
	return nil
}

// DisableSQLitePersistence closes the active usage SQLite database, if configured.
func DisableSQLitePersistence() error {
	prev := defaultSQLitePlugin.swapStore(nil)
	if prev == nil {
		return nil
	}
	log.Infof("usage sqlite persistence disabled: %s", prev.path)
	return prev.Close()
}

// ResolveSQLitePersistencePath returns the SQLite file path used for persisted usage statistics.
func ResolveSQLitePersistencePath(configFilePath string, cfg *config.Config) string {
	for _, key := range []string{"USAGE_SQLITE_PATH", "usage_sqlite_path"} {
		if value, ok := os.LookupEnv(key); ok {
			trimmed := strings.TrimSpace(value)
			switch strings.ToLower(trimmed) {
			case "", "off", "disable", "disabled", "false", "none":
				return ""
			default:
				return filepath.Clean(trimmed)
			}
		}
	}

	baseDir := ""
	if writable := util.WritablePath(); writable != "" {
		baseDir = filepath.Join(writable, "state")
	}
	if baseDir == "" {
		cleaned := strings.TrimSpace(configFilePath)
		if cleaned != "" {
			baseDir = filepath.Dir(cleaned)
			if info, err := os.Stat(cleaned); err == nil && info.IsDir() {
				baseDir = cleaned
			}
			baseDir = filepath.Join(baseDir, "state")
		}
	}
	if baseDir == "" && cfg != nil {
		if authDir, err := util.ResolveAuthDir(cfg.AuthDir); err == nil && strings.TrimSpace(authDir) != "" {
			baseDir = filepath.Join(authDir, "state")
		}
	}
	if baseDir == "" {
		return ""
	}
	return filepath.Join(baseDir, defaultUsageSQLiteFilename)
}

// HandleUsage implements coreusage.Plugin.
func (p *SQLitePlugin) HandleUsage(ctx context.Context, record coreusage.Record) {
	if !statisticsEnabled.Load() {
		return
	}
	store := p.currentStore()
	if store == nil {
		return
	}
	stored := normaliseStoredRecord(ctx, record)
	if err := store.InsertStoredRecord(stored); err != nil {
		log.WithError(err).Warn("usage: failed to persist sqlite statistics")
	}
}

func (p *SQLitePlugin) currentStore() *sqliteStore {
	if p == nil {
		return nil
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.store
}

func (p *SQLitePlugin) swapStore(next *sqliteStore) *sqliteStore {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	prev := p.store
	p.store = next
	return prev
}

type sqliteStore struct {
	path       string
	db         *sql.DB
	insertStmt *sql.Stmt
	retention  sqliteRetentionSettings
	opMu       sync.Mutex
	inserted   int
}

func newSQLiteStore(path string) (*sqliteStore, error) {
	return newSQLiteStoreWithRetention(path, resolveSQLiteRetentionSettings())
}

func newSQLiteStoreWithRetention(path string, retention sqliteRetentionSettings) (*sqliteStore, error) {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" {
		return nil, fmt.Errorf("usage sqlite: path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("usage sqlite: create directory: %w", err)
	}

	db, err := sql.Open(sqliteDriverName, path)
	if err != nil {
		return nil, fmt.Errorf("usage sqlite: open database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	store := &sqliteStore{
		path:      path,
		db:        db,
		retention: retention,
	}
	if err := store.prepare(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *sqliteStore) prepare() error {
	if s == nil || s.db == nil {
		return fmt.Errorf("usage sqlite: database not initialized")
	}
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
	}
	for _, stmt := range pragmas {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("usage sqlite: apply pragma %q: %w", stmt, err)
		}
	}

	schemaStatements := []string{
		`CREATE TABLE IF NOT EXISTS usage_records (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			api_name TEXT NOT NULL,
			model TEXT NOT NULL,
			requested_at TEXT NOT NULL,
			latency_ms INTEGER NOT NULL DEFAULT 0,
			api_key TEXT NOT NULL DEFAULT '',
			source TEXT NOT NULL DEFAULT '',
			auth_index TEXT NOT NULL DEFAULT '',
			failed INTEGER NOT NULL DEFAULT 0,
			input_tokens INTEGER NOT NULL DEFAULT 0,
			output_tokens INTEGER NOT NULL DEFAULT 0,
			reasoning_tokens INTEGER NOT NULL DEFAULT 0,
			cached_tokens INTEGER NOT NULL DEFAULT 0,
			total_tokens INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE (
				api_name,
				model,
				requested_at,
				api_key,
				source,
				auth_index,
				failed,
				input_tokens,
				output_tokens,
				reasoning_tokens,
				cached_tokens,
				total_tokens
			)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_records_requested_at ON usage_records(requested_at)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_records_api_model ON usage_records(api_name, model)`,
	}
	for _, stmt := range schemaStatements {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("usage sqlite: migrate schema: %w", err)
		}
	}

	// Migrate older databases that lack columns added after initial release.
	migrations := []struct {
		column string
		ddl    string
	}{
		{"api_key", "ALTER TABLE usage_records ADD COLUMN api_key TEXT NOT NULL DEFAULT ''"},
		{"source", "ALTER TABLE usage_records ADD COLUMN source TEXT NOT NULL DEFAULT ''"},
		{"auth_index", "ALTER TABLE usage_records ADD COLUMN auth_index TEXT NOT NULL DEFAULT ''"},
		{"failed", "ALTER TABLE usage_records ADD COLUMN failed INTEGER NOT NULL DEFAULT 0"},
		{"input_tokens", "ALTER TABLE usage_records ADD COLUMN input_tokens INTEGER NOT NULL DEFAULT 0"},
		{"output_tokens", "ALTER TABLE usage_records ADD COLUMN output_tokens INTEGER NOT NULL DEFAULT 0"},
		{"reasoning_tokens", "ALTER TABLE usage_records ADD COLUMN reasoning_tokens INTEGER NOT NULL DEFAULT 0"},
		{"cached_tokens", "ALTER TABLE usage_records ADD COLUMN cached_tokens INTEGER NOT NULL DEFAULT 0"},
		{"total_tokens", "ALTER TABLE usage_records ADD COLUMN total_tokens INTEGER NOT NULL DEFAULT 0"},
	}
	for _, m := range migrations {
		var count int
		err := s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('usage_records') WHERE name = ?`, m.column).Scan(&count)
		if err != nil {
			return fmt.Errorf("usage sqlite: check column %s: %w", m.column, err)
		}
		if count == 0 {
			if _, err := s.db.Exec(m.ddl); err != nil {
				return fmt.Errorf("usage sqlite: add column %s: %w", m.column, err)
			}
			log.Infof("usage sqlite: migrated schema — added column %s", m.column)
		}
	}

	// Drop the old UNIQUE constraint and recreate it with all columns if needed.
	// SQLite does not support ALTER TABLE ... DROP CONSTRAINT, so we only log a
	// note; the INSERT OR IGNORE will still work because the new UNIQUE index
	// covers a superset of the old one on freshly-created tables.

	insertStmt, err := s.db.Prepare(`
		INSERT OR IGNORE INTO usage_records (
			api_name,
			model,
			requested_at,
			latency_ms,
			api_key,
			source,
			auth_index,
			failed,
			input_tokens,
			output_tokens,
			reasoning_tokens,
			cached_tokens,
			total_tokens
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("usage sqlite: prepare insert statement: %w", err)
	}
	s.insertStmt = insertStmt
	return nil
}

func (s *sqliteStore) ensureBillingPriceSchema() error {
	if s == nil || s.db == nil {
		return fmt.Errorf("usage sqlite: database not initialized")
	}
	statements := []string{
		`CREATE TABLE IF NOT EXISTS billing_model_prices (
			model_name TEXT PRIMARY KEY,
			input_price_per_m_tokens REAL NOT NULL DEFAULT 0,
			output_price_per_m_tokens REAL NOT NULL DEFAULT 0,
			reasoning_price_per_m_tokens REAL NOT NULL DEFAULT 0,
			cached_price_per_m_tokens REAL NOT NULL DEFAULT 0,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_billing_model_prices_updated_at ON billing_model_prices(updated_at)`,
	}
	for _, stmt := range statements {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("usage sqlite: migrate billing price schema: %w", err)
		}
	}
	return nil
}

func (s *sqliteStore) LoadBillingModelPrices(ctx context.Context) ([]BillingModelPrice, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	if err := s.ensureBillingPriceSchema(); err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT model_name, input_price_per_m_tokens, output_price_per_m_tokens, reasoning_price_per_m_tokens, cached_price_per_m_tokens, updated_at
		FROM billing_model_prices
		ORDER BY model_name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("usage sqlite: query billing prices: %w", err)
	}
	defer rows.Close()
	prices := make([]BillingModelPrice, 0)
	for rows.Next() {
		var item BillingModelPrice
		if err := rows.Scan(&item.ModelName, &item.InputPricePerM, &item.OutputPricePerM, &item.ReasoningPricePerM, &item.CachedPricePerM, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("usage sqlite: scan billing price: %w", err)
		}
		prices = append(prices, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("usage sqlite: iterate billing prices: %w", err)
	}
	return prices, nil
}

func (s *sqliteStore) SaveBillingModelPrices(ctx context.Context, prices []BillingModelPrice) error {
	if s == nil || s.db == nil {
		return nil
	}
	if err := s.ensureBillingPriceSchema(); err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	s.opMu.Lock()
	defer s.opMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("usage sqlite: begin billing price tx: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()
	if _, err := tx.ExecContext(ctx, `DELETE FROM billing_model_prices`); err != nil {
		return fmt.Errorf("usage sqlite: clear billing prices: %w", err)
	}
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO billing_model_prices (
			model_name,
			input_price_per_m_tokens,
			output_price_per_m_tokens,
			reasoning_price_per_m_tokens,
			cached_price_per_m_tokens,
			updated_at
		) VALUES (?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("usage sqlite: prepare billing price statement: %w", err)
	}
	for _, item := range prices {
		if _, err := stmt.ExecContext(ctx, strings.TrimSpace(item.ModelName), item.InputPricePerM, item.OutputPricePerM, item.ReasoningPricePerM, item.CachedPricePerM, formatSQLiteTimestamp(time.Now().UTC())); err != nil {
			_ = stmt.Close()
			return fmt.Errorf("usage sqlite: insert billing price: %w", err)
		}
	}
	if err := stmt.Close(); err != nil {
		return fmt.Errorf("usage sqlite: close billing price statement: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("usage sqlite: commit billing price tx: %w", err)
	}
	tx = nil
	return nil
}

func LoadBillingModelPrices(ctx context.Context, configFilePath string, cfg *config.Config) ([]BillingModelPrice, error) {
	path := ResolveSQLitePersistencePath(configFilePath, cfg)
	if strings.TrimSpace(path) == "" {
		return []BillingModelPrice{}, nil
	}
	store, err := newSQLiteStore(path)
	if err != nil {
		return nil, err
	}
	defer store.Close()
	return store.LoadBillingModelPrices(ctx)
}

func SaveBillingModelPrices(ctx context.Context, configFilePath string, cfg *config.Config, prices []BillingModelPrice) error {
	path := ResolveSQLitePersistencePath(configFilePath, cfg)
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("usage sqlite: billing price persistence path is empty")
	}
	store, err := newSQLiteStore(path)
	if err != nil {
		return err
	}
	defer store.Close()
	return store.SaveBillingModelPrices(ctx, prices)
}

func (s *sqliteStore) Close() error {
	if s == nil {
		return nil
	}
	if s.insertStmt != nil {
		if err := s.insertStmt.Close(); err != nil {
			_ = s.db.Close()
			return err
		}
	}
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *sqliteStore) InsertRecord(ctx context.Context, record coreusage.Record) error {
	stored := normaliseStoredRecord(ctx, record)
	return s.InsertStoredRecord(stored)
}

func (s *sqliteStore) InsertStoredRecord(stored storedUsageRecord) error {
	if s == nil || s.insertStmt == nil {
		return nil
	}
	s.opMu.Lock()
	defer s.opMu.Unlock()

	writeCtx, cancel := context.WithTimeout(context.Background(), sqliteWriteTimeout)
	defer cancel()

	_, err := s.insertStmt.ExecContext(
		writeCtx,
		stored.apiName,
		stored.model,
		formatSQLiteTimestamp(stored.timestamp),
		stored.detail.LatencyMs,
		stored.detail.APIKey,
		stored.detail.Source,
		stored.detail.AuthIndex,
		boolToInt(stored.detail.Failed),
		stored.detail.Tokens.InputTokens,
		stored.detail.Tokens.OutputTokens,
		stored.detail.Tokens.ReasoningTokens,
		stored.detail.Tokens.CachedTokens,
		stored.detail.Tokens.TotalTokens,
	)
	if err != nil {
		return fmt.Errorf("usage sqlite: insert record: %w", err)
	}
	s.inserted++
	if s.shouldPrune() {
		if _, err := s.pruneLocked(writeCtx); err != nil {
			return err
		}
	}
	return nil
}

func (s *sqliteStore) LoadSnapshot(ctx context.Context) (StatisticsSnapshot, error) {
	if s == nil || s.db == nil {
		return StatisticsSnapshot{}, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	s.opMu.Lock()
	defer s.opMu.Unlock()
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			api_name,
			model,
			requested_at,
			latency_ms,
			api_key,
			source,
			auth_index,
			failed,
			input_tokens,
			output_tokens,
			reasoning_tokens,
			cached_tokens,
			total_tokens
		FROM usage_records
		ORDER BY requested_at ASC, id ASC
	`)
	if err != nil {
		return StatisticsSnapshot{}, fmt.Errorf("usage sqlite: query records: %w", err)
	}
	defer rows.Close()

	stats := NewRequestStatistics()
	stats.mu.Lock()

	for rows.Next() {
		var (
			apiName         string
			modelName       string
			requestedAtText string
			latencyMs       int64
			apiKey          string
			source          string
			authIndex       string
			failed          int
			inputTokens     int64
			outputTokens    int64
			reasoningTokens int64
			cachedTokens    int64
			totalTokens     int64
		)
		if err := rows.Scan(
			&apiName,
			&modelName,
			&requestedAtText,
			&latencyMs,
			&apiKey,
			&source,
			&authIndex,
			&failed,
			&inputTokens,
			&outputTokens,
			&reasoningTokens,
			&cachedTokens,
			&totalTokens,
		); err != nil {
			stats.mu.Unlock()
			return StatisticsSnapshot{}, fmt.Errorf("usage sqlite: scan record: %w", err)
		}
		timestamp, err := parseSQLiteTimestamp(requestedAtText)
		if err != nil {
			stats.mu.Unlock()
			return StatisticsSnapshot{}, fmt.Errorf("usage sqlite: parse timestamp %q: %w", requestedAtText, err)
		}

		apiName = strings.TrimSpace(apiName)
		if apiName == "" {
			apiName = "unknown"
		}
		modelName = strings.TrimSpace(modelName)
		if modelName == "" {
			modelName = "unknown"
		}

		apiStatsValue, ok := stats.apis[apiName]
		if !ok {
			apiStatsValue = &apiStats{Models: make(map[string]*modelStats)}
			stats.apis[apiName] = apiStatsValue
		}
		stats.recordImported(apiName, modelName, apiStatsValue, RequestDetail{
			Timestamp: timestamp,
			LatencyMs: latencyMs,
			APIKey:    apiKey,
			Source:    source,
			AuthIndex: authIndex,
			Failed:    failed != 0,
			Tokens: normaliseTokenStats(TokenStats{
				InputTokens:     inputTokens,
				OutputTokens:    outputTokens,
				ReasoningTokens: reasoningTokens,
				CachedTokens:    cachedTokens,
				TotalTokens:     totalTokens,
			}),
		})
	}
	if err := rows.Err(); err != nil {
		stats.mu.Unlock()
		return StatisticsSnapshot{}, fmt.Errorf("usage sqlite: iterate rows: %w", err)
	}
	stats.mu.Unlock()
	return stats.Snapshot(), nil
}

func (s *sqliteStore) Prune(ctx context.Context) (int64, error) {
	if s == nil || s.db == nil {
		return 0, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	s.opMu.Lock()
	defer s.opMu.Unlock()
	return s.pruneLocked(ctx)
}

type storedUsageRecord struct {
	apiName   string
	model     string
	timestamp time.Time
	detail    RequestDetail
}

func normaliseStoredRecord(ctx context.Context, record coreusage.Record) storedUsageRecord {
	timestamp := record.RequestedAt
	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}
	detail := normaliseDetail(record.Detail)
	statsKey := strings.TrimSpace(record.APIKey)
	if statsKey == "" {
		statsKey = resolveAPIIdentifier(ctx, record)
	}
	if statsKey == "" {
		statsKey = "unknown"
	}
	modelName := strings.TrimSpace(record.Model)
	if modelName == "" {
		modelName = "unknown"
	}
	failed := record.Failed
	if !failed {
		failed = !resolveSuccess(ctx)
	}
	return storedUsageRecord{
		apiName:   statsKey,
		model:     modelName,
		timestamp: timestamp,
		detail: RequestDetail{
			Timestamp: timestamp,
			LatencyMs: normaliseLatency(record.Latency),
			APIKey:    statsKey,
			Source:    strings.TrimSpace(record.Source),
			AuthIndex: strings.TrimSpace(record.AuthIndex),
			Failed:    failed,
			Tokens:    detail,
		},
	}
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func formatSQLiteTimestamp(value time.Time) string {
	return value.UTC().Format(sqliteTimestampLayout)
}

func parseSQLiteTimestamp(value string) (time.Time, error) {
	parsed, err := time.Parse(sqliteTimestampLayout, value)
	if err == nil {
		return parsed, nil
	}
	return time.Parse(time.RFC3339Nano, value)
}

func resolveSQLiteRetentionSettings() sqliteRetentionSettings {
	return sqliteRetentionSettings{
		RetentionDays: lookupEnvInt(defaultRetentionDays, "USAGE_SQLITE_RETENTION_DAYS", "usage_sqlite_retention_days"),
		MaxRows:       lookupEnvInt(defaultMaxPersistedRows, "USAGE_SQLITE_MAX_ROWS", "usage_sqlite_max_rows"),
	}
}

func lookupEnvInt(defaultValue int, keys ...string) int {
	for _, key := range keys {
		value, ok := os.LookupEnv(key)
		if !ok {
			continue
		}
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		parsed, err := strconv.Atoi(trimmed)
		if err != nil {
			log.Warnf("usage sqlite: invalid integer for %s=%q, using default %d", key, trimmed, defaultValue)
			return defaultValue
		}
		if parsed <= 0 {
			return 0
		}
		return parsed
	}
	return defaultValue
}

func (s *sqliteStore) shouldPrune() bool {
	if s == nil {
		return false
	}
	if s.retention.RetentionDays <= 0 && s.retention.MaxRows <= 0 {
		return false
	}
	return s.inserted >= sqlitePruneEveryInserts
}

func (s *sqliteStore) pruneLocked(ctx context.Context) (int64, error) {
	if s == nil || s.db == nil {
		return 0, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if s.retention.RetentionDays <= 0 && s.retention.MaxRows <= 0 {
		s.inserted = 0
		return 0, nil
	}

	var totalDeleted int64
	if s.retention.RetentionDays > 0 {
		cutoff := formatSQLiteTimestamp(time.Now().UTC().AddDate(0, 0, -s.retention.RetentionDays))
		result, err := s.db.ExecContext(ctx, `DELETE FROM usage_records WHERE requested_at < ?`, cutoff)
		if err != nil {
			return totalDeleted, fmt.Errorf("usage sqlite: prune by retention days: %w", err)
		}
		deleted, err := result.RowsAffected()
		if err == nil {
			totalDeleted += deleted
		}
	}
	if s.retention.MaxRows > 0 {
		result, err := s.db.ExecContext(ctx, `
			DELETE FROM usage_records
			WHERE id IN (
				SELECT id
				FROM usage_records
				ORDER BY requested_at DESC, id DESC
				LIMIT -1 OFFSET ?
			)
		`, s.retention.MaxRows)
		if err != nil {
			return totalDeleted, fmt.Errorf("usage sqlite: prune by max rows: %w", err)
		}
		deleted, err := result.RowsAffected()
		if err == nil {
			totalDeleted += deleted
		}
	}
	s.inserted = 0
	if totalDeleted > 0 {
		log.Infof(
			"usage sqlite pruned %d record(s) from %s (retention_days=%d max_rows=%d)",
			totalDeleted,
			s.path,
			s.retention.RetentionDays,
			s.retention.MaxRows,
		)
	}
	return totalDeleted, nil
}
