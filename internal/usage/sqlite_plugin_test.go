package usage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	coreusage "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/usage"
)

func TestSQLiteStoreRoundTrip(t *testing.T) {
	store, err := newSQLiteStore(filepath.Join(t.TempDir(), "usage.sqlite"))
	if err != nil {
		t.Fatalf("newSQLiteStore() error = %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("store.Close() error = %v", err)
		}
	}()

	firstTimestamp := time.Date(2026, 4, 6, 8, 30, 0, 0, time.UTC)
	first := coreusage.Record{
		APIKey:      "test-key",
		Model:       "gpt-5.4",
		RequestedAt: firstTimestamp,
		Latency:     1500 * time.Millisecond,
		Source:      "cli",
		AuthIndex:   "0",
		Detail: coreusage.Detail{
			InputTokens:  10,
			OutputTokens: 20,
			TotalTokens:  30,
		},
	}
	second := coreusage.Record{
		Provider:    "openai",
		Model:       "gpt-5.4-mini",
		RequestedAt: firstTimestamp.Add(2 * time.Hour),
		Latency:     200 * time.Millisecond,
		Source:      "batch",
		AuthIndex:   "1",
		Failed:      true,
	}

	if err := store.InsertRecord(context.Background(), first); err != nil {
		t.Fatalf("InsertRecord(first) error = %v", err)
	}
	if err := store.InsertRecord(context.Background(), first); err != nil {
		t.Fatalf("InsertRecord(duplicate) error = %v", err)
	}
	if err := store.InsertRecord(context.Background(), second); err != nil {
		t.Fatalf("InsertRecord(second) error = %v", err)
	}

	snapshot, err := store.LoadSnapshot(context.Background())
	if err != nil {
		t.Fatalf("LoadSnapshot() error = %v", err)
	}

	if snapshot.TotalRequests != 2 {
		t.Fatalf("TotalRequests = %d, want 2", snapshot.TotalRequests)
	}
	if snapshot.SuccessCount != 1 {
		t.Fatalf("SuccessCount = %d, want 1", snapshot.SuccessCount)
	}
	if snapshot.FailureCount != 1 {
		t.Fatalf("FailureCount = %d, want 1", snapshot.FailureCount)
	}
	if snapshot.TotalTokens != 30 {
		t.Fatalf("TotalTokens = %d, want 30", snapshot.TotalTokens)
	}

	keySnapshot, ok := snapshot.APIs["test-key"]
	if !ok {
		t.Fatalf("expected API snapshot for test-key")
	}
	if keySnapshot.TotalRequests != 1 {
		t.Fatalf("test-key TotalRequests = %d, want 1", keySnapshot.TotalRequests)
	}
	modelSnapshot, ok := keySnapshot.Models["gpt-5.4"]
	if !ok {
		t.Fatalf("expected model snapshot for gpt-5.4")
	}
	if len(modelSnapshot.Details) != 1 {
		t.Fatalf("details len = %d, want 1", len(modelSnapshot.Details))
	}
	if modelSnapshot.Details[0].LatencyMs != 1500 {
		t.Fatalf("LatencyMs = %d, want 1500", modelSnapshot.Details[0].LatencyMs)
	}

	providerSnapshot, ok := snapshot.APIs["openai"]
	if !ok {
		t.Fatalf("expected API snapshot for provider-derived key")
	}
	if providerSnapshot.TotalRequests != 1 {
		t.Fatalf("openai TotalRequests = %d, want 1", providerSnapshot.TotalRequests)
	}
	if !providerSnapshot.Models["gpt-5.4-mini"].Details[0].Failed {
		t.Fatalf("expected persisted failed flag for second record")
	}
}

func TestSQLiteStorePruneRespectsMaxRows(t *testing.T) {
	store, err := newSQLiteStoreWithRetention(
		filepath.Join(t.TempDir(), "usage.sqlite"),
		sqliteRetentionSettings{RetentionDays: 0, MaxRows: 2},
	)
	if err != nil {
		t.Fatalf("newSQLiteStoreWithRetention() error = %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("store.Close() error = %v", err)
		}
	}()

	base := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	records := []coreusage.Record{
		{APIKey: "a", Model: "m1", RequestedAt: base, Detail: coreusage.Detail{TotalTokens: 1}},
		{APIKey: "a", Model: "m2", RequestedAt: base.Add(time.Hour), Detail: coreusage.Detail{TotalTokens: 2}},
		{APIKey: "b", Model: "m3", RequestedAt: base.Add(2 * time.Hour), Detail: coreusage.Detail{TotalTokens: 3}},
	}
	for _, record := range records {
		if err := store.InsertRecord(context.Background(), record); err != nil {
			t.Fatalf("InsertRecord() error = %v", err)
		}
	}

	deleted, err := store.Prune(context.Background())
	if err != nil {
		t.Fatalf("Prune() error = %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}

	snapshot, err := store.LoadSnapshot(context.Background())
	if err != nil {
		t.Fatalf("LoadSnapshot() error = %v", err)
	}
	if snapshot.TotalRequests != 2 {
		t.Fatalf("TotalRequests = %d, want 2", snapshot.TotalRequests)
	}
	if _, ok := snapshot.APIs["a"].Models["m1"]; ok {
		t.Fatalf("expected oldest record to be pruned")
	}
}

func TestSQLiteStoreInsertRecordIgnoresCanceledContext(t *testing.T) {
	store, err := newSQLiteStore(filepath.Join(t.TempDir(), "usage.sqlite"))
	if err != nil {
		t.Fatalf("newSQLiteStore() error = %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("store.Close() error = %v", err)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	record := coreusage.Record{
		APIKey:      "canceled-key",
		Model:       "gpt-5.4",
		RequestedAt: time.Date(2026, 4, 6, 9, 0, 0, 0, time.UTC),
		Detail: coreusage.Detail{
			TotalTokens: 42,
		},
	}

	if err := store.InsertRecord(ctx, record); err != nil {
		t.Fatalf("InsertRecord() error = %v", err)
	}

	snapshot, err := store.LoadSnapshot(context.Background())
	if err != nil {
		t.Fatalf("LoadSnapshot() error = %v", err)
	}
	if snapshot.TotalRequests != 1 {
		t.Fatalf("TotalRequests = %d, want 1", snapshot.TotalRequests)
	}
}

func TestEnableSQLitePersistenceRecoversFromMalformedDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage.sqlite")
	if err := os.WriteFile(path, []byte("not a sqlite database"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(main) error = %v", err)
	}
	if err := os.WriteFile(path+"-wal", []byte("wal"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(wal) error = %v", err)
	}
	if err := os.WriteFile(path+"-shm", []byte("shm"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(shm) error = %v", err)
	}

	oldStats := defaultRequestStatistics
	defaultRequestStatistics = NewRequestStatistics()
	defer func() {
		defaultRequestStatistics = oldStats
		if err := DisableSQLitePersistence(); err != nil {
			t.Fatalf("DisableSQLitePersistence() error = %v", err)
		}
	}()

	t.Setenv("USAGE_SQLITE_PATH", path)
	if err := EnableSQLitePersistence(context.Background(), "", nil); err != nil {
		t.Fatalf("EnableSQLitePersistence() error = %v", err)
	}

	matches, err := filepath.Glob(path + ".corrupt-*")
	if err != nil {
		t.Fatalf("filepath.Glob() error = %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("backup main db files = %d, want 1", len(matches))
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("new sqlite database missing: %v", err)
	}

	store := defaultSQLitePlugin.currentStore()
	if store == nil {
		t.Fatalf("expected active sqlite store after recovery")
	}
	record := coreusage.Record{
		APIKey:      "recovered-key",
		Model:       "gpt-5.4",
		RequestedAt: time.Date(2026, 4, 10, 13, 0, 0, 0, time.UTC),
	}
	if err := store.InsertRecord(context.Background(), record); err != nil {
		t.Fatalf("InsertRecord() after recovery error = %v", err)
	}
	snapshot, err := store.LoadSnapshot(context.Background())
	if err != nil {
		t.Fatalf("LoadSnapshot() after recovery error = %v", err)
	}
	if snapshot.TotalRequests != 1 {
		t.Fatalf("TotalRequests = %d, want 1", snapshot.TotalRequests)
	}
}

func TestSQLiteStoreBillingPricesRoundTrip(t *testing.T) {
	store, err := newSQLiteStore(filepath.Join(t.TempDir(), "usage.sqlite"))
	if err != nil {
		t.Fatalf("newSQLiteStore() error = %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("store.Close() error = %v", err)
		}
	}()

	prices := []BillingModelPrice{
		{ModelName: "gpt-5.4", InputPricePerM: 2.5, OutputPricePerM: 10, ReasoningPricePerM: 1.2, CachedPricePerM: 0.25},
		{ModelName: "claude-sonnet-4", InputPricePerM: 3, OutputPricePerM: 15, ReasoningPricePerM: 0, CachedPricePerM: 0.3},
	}
	if err := store.SaveBillingModelPrices(context.Background(), prices); err != nil {
		t.Fatalf("SaveBillingModelPrices() error = %v", err)
	}
	loaded, err := store.LoadBillingModelPrices(context.Background())
	if err != nil {
		t.Fatalf("LoadBillingModelPrices() error = %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("len(loaded) = %d, want 2", len(loaded))
	}
	if loaded[0].ModelName != "claude-sonnet-4" || loaded[1].ModelName != "gpt-5.4" {
		t.Fatalf("unexpected model ordering/content: %#v", loaded)
	}
	if loaded[1].InputPricePerM != 2.5 || loaded[1].OutputPricePerM != 10 {
		t.Fatalf("unexpected loaded pricing: %#v", loaded[1])
	}

	updated := []BillingModelPrice{{ModelName: "gpt-5.4", InputPricePerM: 1, OutputPricePerM: 2}}
	if err := store.SaveBillingModelPrices(context.Background(), updated); err != nil {
		t.Fatalf("SaveBillingModelPrices(update) error = %v", err)
	}
	loaded, err = store.LoadBillingModelPrices(context.Background())
	if err != nil {
		t.Fatalf("LoadBillingModelPrices(after update) error = %v", err)
	}
	if len(loaded) != 1 || loaded[0].ModelName != "gpt-5.4" {
		t.Fatalf("unexpected updated pricing rows: %#v", loaded)
	}
	if loaded[0].InputPricePerM != 1 || loaded[0].OutputPricePerM != 2 {
		t.Fatalf("unexpected updated pricing values: %#v", loaded[0])
	}
}
