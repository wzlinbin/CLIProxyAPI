package p2p

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const requestPoolGinKey = "request_pool"

var errSuspendedUser = errors.New("p2p account is suspended")

func (m *Module) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, apiKey, err := m.authenticate(c.Request)
		if err != nil {
			status := http.StatusUnauthorized
			if errors.Is(err, errSuspendedUser) {
				status = http.StatusForbidden
			}
			c.AbortWithStatusJSON(status, gin.H{"error": err.Error()})
			return
		}

		c.Set("apiKey", apiKey)
		c.Set("accessProvider", "p2p")
		c.Set("accessMetadata", map[string]string{
			"user_id": user.ID,
			"pool":    m.settings.RequestPool,
		})
		_ = m.store.TouchUserLastActive(c.Request.Context(), user.ID)
		c.Next()
	}
}

func (m *Module) requestPoolMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(requestPoolGinKey, m.settings.RequestPool)
		c.Next()
	}
}

func (m *Module) authenticate(r *http.Request) (*User, string, error) {
	apiKey := extractAPIKey(r)
	if apiKey == "" {
		return nil, "", errors.New("missing p2p api key")
	}

	user, err := m.store.GetUserByAPIKey(r.Context(), apiKey)
	if err != nil {
		return nil, "", errors.New("invalid p2p api key")
	}

	switch user.Status {
	case UserStatusActive:
		return user, apiKey, nil
	case UserStatusSuspended:
		return nil, "", errSuspendedUser
	default:
		return nil, "", errors.New("p2p account is disabled")
	}
}

func extractAPIKey(r *http.Request) string {
	if r == nil {
		return ""
	}

	candidates := []string{
		extractBearerToken(r.Header.Get("Authorization")),
		r.Header.Get("X-API-Key"),
		r.Header.Get("X-Goog-Api-Key"),
	}
	if r.URL != nil {
		candidates = append(candidates, r.URL.Query().Get("key"), r.URL.Query().Get("api_key"))
	}
	for _, item := range candidates {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func extractBearerToken(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parts := strings.SplitN(raw, " ", 2)
	if len(parts) != 2 {
		return raw
	}
	if !strings.EqualFold(parts[0], "bearer") {
		return raw
	}
	return strings.TrimSpace(parts[1])
}
