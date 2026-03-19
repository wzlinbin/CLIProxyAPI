package p2p

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Verifier struct {
	client *http.Client
}

func NewVerifier() *Verifier {
	return &Verifier{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (v *Verifier) VerifyProvider(ctx context.Context, provider *UserProvider) *VerificationResult {
	result := &VerificationResult{}
	if provider == nil {
		result.ErrorMessage = "provider is nil"
		return result
	}

	baseURL := normalizeBaseURL(provider.ProviderType, provider.BaseURL)
	models, err := v.listModels(ctx, provider.ProviderType, baseURL, provider.APIKey)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("failed to list models: %v", err)
		return result
	}
	result.Models = models

	ok, err := v.testRequest(ctx, provider.ProviderType, baseURL, provider.APIKey, models)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("test request failed: %v", err)
		return result
	}
	result.Success = ok
	result.TestPassed = ok
	return result
}

func (v *Verifier) listModels(ctx context.Context, providerType ProviderType, baseURL, apiKey string) ([]string, error) {
	switch providerType {
	case ProviderTypeClaude:
		return []string{
			"claude-3-5-haiku-20241022",
			"claude-3-5-sonnet-20241022",
			"claude-3-7-sonnet-20250219",
		}, nil
	case ProviderTypeOpenAI, ProviderTypeCodex, ProviderTypeQwen:
		return v.listOpenAIModels(ctx, strings.TrimRight(baseURL, "/")+"/models", apiKey)
	case ProviderTypeGemini:
		url := strings.TrimRight(baseURL, "/") + "/models?key=" + apiKey
		return v.listGeminiModels(ctx, url)
	default:
		return nil, fmt.Errorf("unsupported provider type %q", providerType)
	}
}

func (v *Verifier) listOpenAIModels(ctx context.Context, modelsURL, apiKey string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(apiKey))

	resp, err := v.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	models := make([]string, 0, len(payload.Data))
	for _, item := range payload.Data {
		if trimmed := strings.TrimSpace(item.ID); trimmed != "" {
			models = append(models, trimmed)
		}
	}
	return normalizeModelIDs(models), nil
}

func (v *Verifier) listGeminiModels(ctx context.Context, modelsURL string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := v.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	models := make([]string, 0, len(payload.Models))
	for _, item := range payload.Models {
		trimmed := strings.TrimSpace(strings.TrimPrefix(item.Name, "models/"))
		if trimmed != "" {
			models = append(models, trimmed)
		}
	}
	return normalizeModelIDs(models), nil
}

func (v *Verifier) testRequest(ctx context.Context, providerType ProviderType, baseURL, apiKey string, models []string) (bool, error) {
	if len(models) == 0 {
		return false, fmt.Errorf("no models detected")
	}
	model := selectTestModel(models)

	switch providerType {
	case ProviderTypeOpenAI, ProviderTypeCodex, ProviderTypeQwen:
		return v.testOpenAICompat(ctx, strings.TrimRight(baseURL, "/")+"/chat/completions", apiKey, model)
	case ProviderTypeClaude:
		return v.testClaude(ctx, strings.TrimRight(baseURL, "/")+"/messages", apiKey, model)
	case ProviderTypeGemini:
		url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", strings.TrimRight(baseURL, "/"), model, apiKey)
		return v.testGemini(ctx, url)
	default:
		return false, fmt.Errorf("unsupported provider type %q", providerType)
	}
}

func selectTestModel(models []string) string {
	preferred := []string{
		"gpt-4o-mini",
		"gpt-4.1-mini",
		"claude-3-5-haiku",
		"claude-3-5-sonnet",
		"gemini-2.0-flash",
		"gemini-1.5-flash",
		"qwen-plus",
	}
	for _, target := range preferred {
		for _, model := range models {
			if strings.Contains(strings.ToLower(model), strings.ToLower(target)) {
				return model
			}
		}
	}
	return models[0]
}

func (v *Verifier) testOpenAICompat(ctx context.Context, endpoint, apiKey, model string) (bool, error) {
	body, _ := json.Marshal(map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": "Reply with the single word test."},
		},
		"max_tokens": 8,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(apiKey))
	req.Header.Set("Content-Type", "application/json")

	resp, err := v.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode >= 200 && resp.StatusCode < 300, nil
}

func (v *Verifier) testClaude(ctx context.Context, endpoint, apiKey, model string) (bool, error) {
	body, _ := json.Marshal(map[string]any{
		"model":      model,
		"max_tokens": 8,
		"messages": []map[string]string{
			{"role": "user", "content": "Reply with the single word test."},
		},
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return false, err
	}
	req.Header.Set("x-api-key", strings.TrimSpace(apiKey))
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")

	resp, err := v.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode >= 200 && resp.StatusCode < 300, nil
}

func (v *Verifier) testGemini(ctx context.Context, endpoint string) (bool, error) {
	body, _ := json.Marshal(map[string]any{
		"contents": []map[string]any{
			{
				"parts": []map[string]string{
					{"text": "Reply with the single word test."},
				},
			},
		},
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := v.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode >= 200 && resp.StatusCode < 300, nil
}
