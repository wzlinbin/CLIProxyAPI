package p2p

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handlers struct {
	module *Module
}

func NewHandlers(module *Module) *Handlers {
	return &Handlers{module: module}
}

func (h *Handlers) RegisterRoutes(r *gin.Engine) {
	group := r.Group("/p2p")
	{
		group.GET("", h.ServeFrontend)
		group.GET("/", h.ServeFrontend)
		group.POST("/register", h.Register)
		group.GET("/info", h.GetInfo)
		group.GET("/stats", h.GetStats)
		group.GET("/models", h.GetAvailableModels)
		group.GET("/overview", h.GetOverview)
	}
}

func (h *Handlers) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.APIKey = strings.TrimSpace(req.APIKey)
	req.BaseURL = strings.TrimSpace(req.BaseURL)
	req.Models = normalizeModelIDs(req.Models)
	if req.ProviderType == "" || req.Name == "" || req.APIKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "provider_type, name, and api_key are required"})
		return
	}

	userAPIKey := "sk-p2p-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:24]

	ctx := c.Request.Context()
	user, err := h.module.store.CreateUser(ctx, req.Email, userAPIKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create p2p user"})
		return
	}

	provider := &UserProvider{
		ID:              uuid.NewString(),
		UserID:          user.ID,
		ProviderType:    req.ProviderType,
		Name:            req.Name,
		BaseURL:         normalizeBaseURL(req.ProviderType, req.BaseURL),
		APIKey:          req.APIKey,
		Models:          req.Models,
		DailyTokenLimit: req.DailyTokenLimit,
		Status:          ProviderStatusPending,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}

	if err := h.module.store.CreateProvider(ctx, provider); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create p2p provider"})
		return
	}

	go h.module.verifyProvider(provider)

	c.JSON(http.StatusOK, RegisterResponse{
		Success:              true,
		Message:              "Registration accepted. Provider verification is running in the background.",
		UserID:               user.ID,
		APIKey:               userAPIKey,
		RequiresVerification: true,
	})
}

func (h *Handlers) GetInfo(c *gin.Context) {
	user, err := h.loadUserFromRequest(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	providers, err := h.module.store.GetProvidersByUserID(ctx, user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load providers"})
		return
	}
	stats, err := h.module.store.GetUserStatsSummary(ctx, user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load stats"})
		return
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
	user, err := h.loadUserFromRequest(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	stats, err := h.module.store.GetUserStatsSummary(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load stats"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

func (h *Handlers) GetAvailableModels(c *gin.Context) {
	models, err := h.module.sharedModels(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load shared models"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"models": models})
}

func (h *Handlers) GetOverview(c *gin.Context) {
	overview, err := h.module.store.GetPlatformOverview(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load platform overview"})
		return
	}
	c.JSON(http.StatusOK, overview)
}

func (h *Handlers) ServeFrontend(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, frontendHTML)
}

func (h *Handlers) loadUserFromRequest(c *gin.Context) (*User, error) {
	apiKey := strings.TrimSpace(c.GetHeader("X-API-Key"))
	if apiKey == "" {
		apiKey = strings.TrimSpace(c.Query("api_key"))
	}
	if apiKey == "" {
		return nil, errors.New("api key required")
	}

	user, err := h.module.store.GetUserByAPIKey(c.Request.Context(), apiKey)
	if err != nil {
		return nil, errors.New("invalid api key")
	}
	return user, nil
}

func backgroundContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 60*time.Second)
}
