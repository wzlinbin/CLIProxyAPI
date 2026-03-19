package auth

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
)

type requestPoolTestExecutor struct {
	id string
}

func (e *requestPoolTestExecutor) Identifier() string { return e.id }
func (e *requestPoolTestExecutor) Execute(context.Context, *Auth, cliproxyexecutor.Request, cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	return cliproxyexecutor.Response{}, nil
}
func (e *requestPoolTestExecutor) ExecuteStream(context.Context, *Auth, cliproxyexecutor.Request, cliproxyexecutor.Options) (*cliproxyexecutor.StreamResult, error) {
	return nil, nil
}
func (e *requestPoolTestExecutor) Refresh(context.Context, *Auth) (*Auth, error) { return nil, nil }
func (e *requestPoolTestExecutor) CountTokens(context.Context, *Auth, cliproxyexecutor.Request, cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	return cliproxyexecutor.Response{}, nil
}
func (e *requestPoolTestExecutor) HttpRequest(context.Context, *Auth, *http.Request) (*http.Response, error) {
	return nil, nil
}

func TestManagerPickNextMixed_DefaultPoolExcludesP2PAuths(t *testing.T) {
	manager, cleanup := newRequestPoolTestManager(t)
	defer cleanup()

	auth, _, provider, err := manager.pickNextMixed(
		context.Background(),
		[]string{"gemini"},
		"gemini-2.0-flash",
		cliproxyexecutor.Options{},
		map[string]struct{}{},
	)
	if err != nil {
		t.Fatalf("pickNextMixed() error = %v", err)
	}
	if auth == nil {
		t.Fatal("pickNextMixed() returned nil auth")
	}
	if auth.ID != "default-auth" {
		t.Fatalf("pickNextMixed() auth.ID = %q, want %q", auth.ID, "default-auth")
	}
	if provider != "gemini" {
		t.Fatalf("pickNextMixed() provider = %q, want %q", provider, "gemini")
	}
}

func TestManagerPickNextMixed_P2PPoolOnlySelectsP2PAuths(t *testing.T) {
	manager, cleanup := newRequestPoolTestManager(t)
	defer cleanup()

	auth, _, provider, err := manager.pickNextMixed(
		context.Background(),
		[]string{"gemini"},
		"gemini-2.0-flash",
		cliproxyexecutor.Options{
			Metadata: map[string]any{
				cliproxyexecutor.RequestPoolMetadataKey: "p2p",
			},
		},
		map[string]struct{}{},
	)
	if err != nil {
		t.Fatalf("pickNextMixed() error = %v", err)
	}
	if auth == nil {
		t.Fatal("pickNextMixed() returned nil auth")
	}
	if auth.ID != "p2p-auth" {
		t.Fatalf("pickNextMixed() auth.ID = %q, want %q", auth.ID, "p2p-auth")
	}
	if provider != "gemini" {
		t.Fatalf("pickNextMixed() provider = %q, want %q", provider, "gemini")
	}
}

func newRequestPoolTestManager(t *testing.T) (*Manager, func()) {
	t.Helper()

	manager := NewManager(nil, &RoundRobinSelector{}, nil)
	manager.RegisterExecutor(&requestPoolTestExecutor{id: "gemini"})

	now := time.Now().UTC()
	defaultAuth := &Auth{
		ID:        "default-auth",
		Provider:  "gemini",
		Status:    StatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
	p2pAuth := &Auth{
		ID:        "p2p-auth",
		Provider:  "gemini",
		Status:    StatusActive,
		CreatedAt: now,
		UpdatedAt: now,
		Attributes: map[string]string{
			"request_pool": "p2p",
		},
	}

	if _, err := manager.Register(context.Background(), defaultAuth); err != nil {
		t.Fatalf("register default auth: %v", err)
	}
	if _, err := manager.Register(context.Background(), p2pAuth); err != nil {
		t.Fatalf("register p2p auth: %v", err)
	}

	model := &registry.ModelInfo{ID: "gemini-2.0-flash", Object: "model", Created: now.Unix(), OwnedBy: "google", Type: "gemini"}
	globalRegistry := registry.GetGlobalRegistry()
	globalRegistry.RegisterClient(defaultAuth.ID, "gemini", []*registry.ModelInfo{model})
	globalRegistry.RegisterClient(p2pAuth.ID, "gemini", []*registry.ModelInfo{model})
	manager.RefreshSchedulerEntry(defaultAuth.ID)
	manager.RefreshSchedulerEntry(p2pAuth.ID)

	cleanup := func() {
		globalRegistry.UnregisterClient(defaultAuth.ID)
		globalRegistry.UnregisterClient(p2pAuth.ID)
	}
	return manager, cleanup
}
