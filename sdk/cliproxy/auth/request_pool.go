package auth

import (
	"strings"

	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
)

func requestedPoolFromMetadata(meta map[string]any) string {
	if len(meta) == 0 {
		return ""
	}
	raw, ok := meta[cliproxyexecutor.RequestPoolMetadataKey]
	if !ok || raw == nil {
		return ""
	}
	switch value := raw.(type) {
	case string:
		return strings.TrimSpace(value)
	case []byte:
		return strings.TrimSpace(string(value))
	default:
		return ""
	}
}

func authRequestPool(auth *Auth) string {
	if auth == nil || auth.Attributes == nil {
		return ""
	}
	return strings.TrimSpace(auth.Attributes["request_pool"])
}

func authMatchesRequestedPool(auth *Auth, requestedPool string) bool {
	authPool := authRequestPool(auth)
	if requestedPool == "" {
		return authPool == ""
	}
	return authPool == requestedPool
}
