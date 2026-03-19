package p2p

import (
	"context"
	"strings"
	"time"

	coreusage "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/usage"
	log "github.com/sirupsen/logrus"
)

type UsagePlugin struct {
	module *Module
}

func (p *UsagePlugin) HandleUsage(ctx context.Context, record coreusage.Record) {
	if p == nil || p.module == nil {
		return
	}

	provider, ok := p.module.authBinding(record.AuthID)
	if !ok {
		return
	}

	apiKey := strings.TrimSpace(record.APIKey)
	if apiKey == "" {
		return
	}

	user, err := p.module.store.GetUserByAPIKey(ctx, apiKey)
	if err != nil || user == nil {
		return
	}

	usageRecord := &UsageRecord{
		UserID:         user.ID,
		ProviderUserID: provider.UserID,
		ProviderID:     provider.ID,
		Model:          strings.TrimSpace(record.Model),
		RequestTokens:  record.Detail.InputTokens,
		ResponseTokens: record.Detail.OutputTokens,
		TotalTokens:    record.Detail.TotalTokens,
		Success:        !record.Failed,
		CreatedAt:      record.RequestedAt.UTC(),
	}
	if usageRecord.CreatedAt.IsZero() {
		usageRecord.CreatedAt = time.Now().UTC()
	}

	if err := p.module.store.RecordUsageTransfer(ctx, usageRecord); err != nil {
		log.WithError(err).WithFields(log.Fields{
			"user_id":     user.ID,
			"provider_id": provider.ID,
			"auth_id":     record.AuthID,
		}).Error("p2p: failed to persist usage record")
	}
}
