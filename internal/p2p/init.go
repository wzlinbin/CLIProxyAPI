package p2p

import (
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultRequestPool        = "p2p"
	defaultSyncInterval       = time.Minute
	defaultEnforcement        = time.Hour
	defaultSuspendRatio       = 1.2
	envP2PDSN                 = "P2P_PG_DSN"
	envP2PAltDSN              = "P2P_POSTGRES_DSN"
	envP2PSyncInterval        = "P2P_SYNC_INTERVAL"
	envP2PEnforcementInterval = "P2P_ENFORCEMENT_INTERVAL"
	envP2PSuspendRatio        = "P2P_SUSPEND_RATIO"
	envP2PRequestPool         = "P2P_REQUEST_POOL"
)

type Settings struct {
	DSN                 string
	RequestPool         string
	SyncInterval        time.Duration
	EnforcementInterval time.Duration
	SuspendRatio        float64
}

func SettingsFromEnv() Settings {
	settings := Settings{
		RequestPool:         defaultRequestPool,
		SyncInterval:        defaultSyncInterval,
		EnforcementInterval: defaultEnforcement,
		SuspendRatio:        defaultSuspendRatio,
	}

	settings.DSN = firstNonEmptyEnv(envP2PDSN, envP2PAltDSN)
	if raw := firstNonEmptyEnv(envP2PRequestPool); raw != "" {
		settings.RequestPool = strings.TrimSpace(raw)
	}
	if settings.RequestPool == "" {
		settings.RequestPool = defaultRequestPool
	}
	if parsed, ok := durationFromEnv(envP2PSyncInterval); ok {
		settings.SyncInterval = parsed
	}
	if parsed, ok := durationFromEnv(envP2PEnforcementInterval); ok {
		settings.EnforcementInterval = parsed
	}
	if raw := firstNonEmptyEnv(envP2PSuspendRatio); raw != "" {
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(raw), 64); err == nil && parsed > 0 {
			settings.SuspendRatio = parsed
		}
	}

	return settings
}

func (s Settings) Enabled() bool {
	return strings.TrimSpace(s.DSN) != ""
}

func firstNonEmptyEnv(keys ...string) string {
	for _, key := range keys {
		value := strings.TrimSpace(os.Getenv(key))
		if value != "" {
			return value
		}
	}
	return ""
}

func durationFromEnv(key string) (time.Duration, bool) {
	raw := firstNonEmptyEnv(key)
	if raw == "" {
		return 0, false
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil || parsed <= 0 {
		return 0, false
	}
	return parsed, true
}
