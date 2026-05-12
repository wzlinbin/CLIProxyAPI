package p2p

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/registry"
	sdkhandlers "github.com/router-for-me/CLIProxyAPI/v7/sdk/api/handlers"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/api/handlers/claude"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/api/handlers/gemini"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/api/handlers/openai"
	coreauth "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/auth"
	coreusage "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/usage"
	log "github.com/sirupsen/logrus"
)

type upsertAuthFunc func(context.Context, *coreauth.Auth)
type removeAuthFunc func(context.Context, string)

type Module struct {
	settings Settings
	store    *P2PStore
	verifier *Verifier
	handlers *Handlers

	mu           sync.RWMutex
	authBindings map[string]UserProvider
	upsert       upsertAuthFunc
	remove       removeAuthFunc
	usagePlugin  *UsagePlugin
	syncCh       chan struct{}
	started      bool
}

func NewModule(ctx context.Context, settings Settings) (*Module, error) {
	if !settings.Enabled() {
		return nil, nil
	}

	store, err := NewP2PStore(ctx, settings.DSN)
	if err != nil {
		return nil, err
	}

	module := &Module{
		settings:     settings,
		store:        store,
		verifier:     NewVerifier(),
		authBindings: make(map[string]UserProvider),
		syncCh:       make(chan struct{}, 1),
	}
	module.handlers = NewHandlers(module)
	module.usagePlugin = &UsagePlugin{module: module}
	return module, nil
}

func (m *Module) Close() {
	if m == nil {
		return
	}
	m.store.Close()
}

func (m *Module) UsagePlugin() coreusage.Plugin {
	if m == nil {
		return nil
	}
	return m.usagePlugin
}

func (m *Module) RegisterRoutes(r *gin.Engine, base *sdkhandlers.BaseAPIHandler) {
	if m == nil || r == nil || base == nil {
		return
	}

	m.handlers.RegisterRoutes(r)

	openaiHandlers := openai.NewOpenAIAPIHandler(base)
	openaiResponsesHandlers := openai.NewOpenAIResponsesAPIHandler(base)
	claudeHandlers := claude.NewClaudeCodeAPIHandler(base)
	geminiHandlers := gemini.NewGeminiAPIHandler(base)

	authMiddleware := m.authMiddleware()
	poolMiddleware := m.requestPoolMiddleware()

	sharedV1 := r.Group("/p2p/v1")
	sharedV1.Use(authMiddleware, poolMiddleware)
	{
		sharedV1.GET("", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"message":      "P2P shared pool API",
				"request_pool": m.settings.RequestPool,
				"models_path":  "/p2p/v1/models",
			})
		})
		sharedV1.GET("/models", m.openAIModels)
		sharedV1.POST("/chat/completions", openaiHandlers.ChatCompletions)
		sharedV1.POST("/completions", openaiHandlers.Completions)
		sharedV1.POST("/messages", claudeHandlers.ClaudeMessages)
		sharedV1.POST("/messages/count_tokens", claudeHandlers.ClaudeCountTokens)
		sharedV1.GET("/responses", openaiResponsesHandlers.ResponsesWebsocket)
		sharedV1.POST("/responses", openaiResponsesHandlers.Responses)
		sharedV1.POST("/responses/compact", openaiResponsesHandlers.Compact)
	}

	sharedV1beta := r.Group("/p2p/v1beta")
	sharedV1beta.Use(authMiddleware, poolMiddleware)
	{
		sharedV1beta.GET("/models", m.geminiModels)
		sharedV1beta.POST("/models/*action", geminiHandlers.GeminiHandler)
		sharedV1beta.GET("/models/*action", geminiHandlers.GeminiGetHandler)
	}
}

func (m *Module) Start(ctx context.Context, upsert upsertAuthFunc, remove removeAuthFunc) error {
	if m == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if upsert == nil || remove == nil {
		return fmt.Errorf("p2p runtime callbacks are required")
	}
	if m.settings.SyncInterval <= 0 {
		m.settings.SyncInterval = defaultSyncInterval
	}
	if m.settings.EnforcementInterval <= 0 {
		m.settings.EnforcementInterval = defaultEnforcement
	}
	if m.settings.SuspendRatio <= 0 {
		m.settings.SuspendRatio = defaultSuspendRatio
	}
	m.upsert = upsert
	m.remove = remove

	if err := m.syncRuntimeAuths(context.Background()); err != nil {
		return err
	}

	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return nil
	}
	m.started = true
	m.mu.Unlock()

	go m.runSyncLoop(ctx)
	go m.runEnforcementLoop(ctx)
	return nil
}

func (m *Module) TriggerSync() {
	if m == nil {
		return
	}
	select {
	case m.syncCh <- struct{}{}:
	default:
	}
}

func (m *Module) verifyProvider(provider *UserProvider) {
	if provider == nil {
		return
	}
	ctx, cancel := backgroundContext()
	defer cancel()

	result := m.verifier.VerifyProvider(ctx, provider)
	status := ProviderStatusFailed
	models := provider.Models
	lastError := ""
	if result.Success {
		status = ProviderStatusVerified
		models = normalizeModelIDs(result.Models)
	} else {
		lastError = strings.TrimSpace(result.ErrorMessage)
	}

	if err := m.store.UpdateProviderVerification(ctx, provider.ID, status, models, lastError); err != nil {
		log.WithError(err).WithField("provider_id", provider.ID).Error("p2p: failed to update provider verification state")
		return
	}
	m.TriggerSync()
}

func (m *Module) sharedModels(ctx context.Context) ([]string, error) {
	providers, err := m.store.GetVerifiedProviders(ctx)
	if err != nil {
		return nil, err
	}
	models := uniqueModelsFromProviders(providers)
	sort.Strings(models)
	return models, nil
}

func (m *Module) authBinding(authID string) (UserProvider, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	provider, ok := m.authBindings[strings.TrimSpace(authID)]
	return provider, ok
}

func (m *Module) runtimeAuthID(providerID string) string {
	return "p2p-provider:" + strings.TrimSpace(providerID)
}

func (m *Module) runtimeProviderKey(providerType ProviderType) string {
	switch providerType {
	case ProviderTypeOpenAI:
		return "p2p-openai"
	case ProviderTypeClaude:
		return "claude"
	case ProviderTypeGemini:
		return "gemini"
	case ProviderTypeCodex:
		return "codex"
	case ProviderTypeQwen:
		return "qwen"
	default:
		return "p2p-openai"
	}
}

func (m *Module) buildRuntimeAuth(provider UserProvider) *coreauth.Auth {
	models := strings.Join(normalizeModelIDs(provider.Models), ",")
	now := time.Now().UTC()
	return &coreauth.Auth{
		ID:       m.runtimeAuthID(provider.ID),
		Provider: m.runtimeProviderKey(provider.ProviderType),
		Label:    "p2p:" + provider.Name,
		Status:   coreauth.StatusActive,
		Attributes: map[string]string{
			"runtime_only":    "true",
			"auth_kind":       "apikey",
			"request_pool":    m.settings.RequestPool,
			"source":          "p2p",
			"p2p_provider_id": provider.ID,
			"p2p_user_id":     provider.UserID,
			"provider_type":   string(provider.ProviderType),
			"api_key":         provider.APIKey,
			"base_url":        normalizeBaseURL(provider.ProviderType, provider.BaseURL),
			"runtime_models":  models,
		},
		Metadata: map[string]any{
			"p2p_provider_name": provider.Name,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func (m *Module) syncRuntimeAuths(ctx context.Context) error {
	providers, err := m.store.GetVerifiedProviders(ctx)
	if err != nil {
		return err
	}

	desired := make(map[string]UserProvider, len(providers))
	for _, provider := range providers {
		authID := m.runtimeAuthID(provider.ID)
		desired[authID] = provider
		if m.upsert != nil {
			m.upsert(context.Background(), m.buildRuntimeAuth(provider))
		}
	}

	m.mu.Lock()
	for authID, provider := range desired {
		m.authBindings[authID] = provider
	}
	stale := make([]string, 0)
	for authID := range m.authBindings {
		if _, exists := desired[authID]; !exists {
			stale = append(stale, authID)
		}
	}
	for _, authID := range stale {
		delete(m.authBindings, authID)
	}
	m.mu.Unlock()

	for _, authID := range stale {
		if m.remove != nil {
			m.remove(context.Background(), authID)
		}
	}

	return nil
}

func (m *Module) runSyncLoop(ctx context.Context) {
	ticker := time.NewTicker(m.settings.SyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-m.syncCh:
		}

		if err := m.syncRuntimeAuths(context.Background()); err != nil {
			log.WithError(err).Error("p2p: runtime auth sync failed")
		}
	}
}

func (m *Module) runEnforcementLoop(ctx context.Context) {
	ticker := time.NewTicker(m.settings.EnforcementInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			count, err := m.store.CheckAndSuspendOverLimitUsers(context.Background(), m.settings.SuspendRatio)
			if err != nil {
				log.WithError(err).Error("p2p: failed to enforce ratio guard")
				continue
			}
			if count > 0 {
				log.WithField("suspended", count).Info("p2p: suspended over-limit users")
			}
		}
	}
}

func (m *Module) openAIModels(c *gin.Context) {
	models, err := m.sharedModels(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load models"})
		return
	}

	now := time.Now().Unix()
	data := make([]map[string]any, 0, len(models))
	for _, model := range models {
		info := registry.LookupStaticModelInfo(model)
		if info != nil {
			data = append(data, map[string]any{
				"id":       info.ID,
				"object":   "model",
				"created":  info.Created,
				"owned_by": info.OwnedBy,
			})
			continue
		}
		data = append(data, map[string]any{
			"id":       model,
			"object":   "model",
			"created":  now,
			"owned_by": "p2p",
		})
	}

	c.JSON(http.StatusOK, gin.H{"object": "list", "data": data})
}

func (m *Module) geminiModels(c *gin.Context) {
	models, err := m.sharedModels(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load models"})
		return
	}

	out := make([]map[string]any, 0, len(models))
	for _, model := range models {
		info := registry.LookupStaticModelInfo(model)
		name := "models/" + model
		displayName := model
		description := model
		if info != nil {
			if strings.TrimSpace(info.Name) != "" {
				name = info.Name
			}
			if strings.TrimSpace(info.DisplayName) != "" {
				displayName = info.DisplayName
			}
			if strings.TrimSpace(info.Description) != "" {
				description = info.Description
			}
		}
		out = append(out, map[string]any{
			"name":                       name,
			"displayName":                displayName,
			"description":                description,
			"supportedGenerationMethods": []string{"generateContent"},
		})
	}

	c.JSON(http.StatusOK, gin.H{"models": out})
}
