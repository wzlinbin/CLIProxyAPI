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

	log "github.com/sirupsen/logrus"
)

type Verifier struct {
	client *http.Client
}

func NewVerifier() *Verifier {
	return &Verifier{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (v *Verifier) VerifyProvider(ctx context.Context, provider *UserProvider) *VerificationResult {
	result := &VerificationResult{}

	baseURL := provider.BaseURL
	if baseURL == "" {
		baseURL = v.getDefaultBaseURL(provider.ProviderType)
	}

	// Step 1: List models
	models, err := v.listModels(ctx, provider.ProviderType, baseURL, provider.APIKey)
	if err != nil {
		result.Success = false
		result.ErrorMessage = fmt.Sprintf("Failed to list models: %v", err)
		return result
	}
	result.Models = models

	// Step 2: Test request
	testPassed, err := v.testRequest(ctx, provider.ProviderType, baseURL, provider.APIKey, models)
	if err != nil {
		result.Success = false
		result.ErrorMessage = fmt.Sprintf("Test request failed: %v", err)
		return result
	}
	result.TestPassed = testPassed

	result.Success = true
	return result
}

func (v *Verifier) getDefaultBaseURL(providerType ProviderType) string {
	switch providerType {
	case ProviderTypeOpenAI:
		return "https://api.openai.com/v1"
	case ProviderTypeClaude:
		return "https://api.anthropic.com/v1"
	case ProviderTypeGemini:
		return "https://generativelanguage.googleapis.com/v1beta"
	case ProviderTypeCodex:
		return "https://api.openai.com/v1"
	case ProviderTypeQwen:
		return "https://dashscope.aliyuncs.com/api/v1"
	default:
		return ""
	}
}

func (v *Verifier) listModels(ctx context.Context, providerType ProviderType, baseURL, apiKey string) ([]string, error) {
	var modelsURL string
	var headers map[string]string

	switch providerType {
	case ProviderTypeOpenAI, ProviderTypeCodex:
		modelsURL = baseURL + "/models"
		headers = map[string]string{"Authorization": "Bearer " + apiKey}
	case ProviderTypeClaude:
		return []string{"claude-3-5-sonnet-20241022", "claude-3-opus-20240229", "claude-3-haiku-20240307"}, nil
	case ProviderTypeGemini:
		modelsURL = baseURL + "/models?key=" + apiKey
		headers = map[string]string{}
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", providerType)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", modelsURL, nil)
	if err != nil {
		return nil, err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := v.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return v.parseModelsResponse(providerType, body)
}

func (v *Verifier) parseModelsResponse(providerType ProviderType, body []byte) ([]string, error) {
	var models []string

	switch providerType {
	case ProviderTypeOpenAI, ProviderTypeCodex:
		var resp struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, err
		}
		for _, m := range resp.Data {
			models = append(models, m.ID)
		}
	case ProviderTypeGemini:
		var resp struct {
			Models []struct {
				Name string `json:"name"`
			} `json:"models"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, err
		}
		for _, m := range resp.Models {
			name := strings.TrimPrefix(m.Name, "models/")
			models = append(models, name)
		}
	}

	return models, nil
}

func (v *Verifier) testRequest(ctx context.Context, providerType ProviderType, baseURL, apiKey string, models []string) (bool, error) {
	if len(models) == 0 {
		return false, fmt.Errorf("no models available for testing")
	}

	testModel := v.selectTestModel(providerType, models)

	switch providerType {
	case ProviderTypeOpenAI, ProviderTypeCodex:
		return v.testOpenAI(ctx, baseURL, apiKey, testModel)
	case ProviderTypeClaude:
		return v.testClaude(ctx, baseURL, apiKey, testModel)
	case ProviderTypeGemini:
		return v.testGemini(ctx, baseURL, apiKey, testModel)
	default:
		return false, fmt.Errorf("unsupported provider type: %s", providerType)
	}
}

func (v *Verifier) selectTestModel(providerType ProviderType, models []string) string {
	preferred := []string{
		"gpt-4o-mini", "gpt-3.5-turbo", "gpt-4o",
		"claude-3-5-haiku-20241022", "claude-3-haiku-20240307",
		"gemini-2.0-flash", "gemini-1.5-flash", "gemini-pro",
	}

	for _, p := range preferred {
		for _, m := range models {
			if strings.Contains(m, p) || m == p {
				return m
			}
		}
	}

	return models[0]
}

func (v *Verifier) testOpenAI(ctx context.Context, baseURL, apiKey, model string) (bool, error) {
	reqBody := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": "Say 'test'"},
		},
		"max_tokens": 5,
	}

	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return false, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := v.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

func (v *Verifier) testClaude(ctx context.Context, baseURL, apiKey, model string) (bool, error) {
	reqBody := map[string]interface{}{
		"model":      model,
		"max_tokens": 5,
		"messages": []map[string]string{
			{"role": "user", "content": "Say 'test'"},
		},
	}

	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return false, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := v.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

func (v *Verifier) testGemini(ctx context.Context, baseURL, apiKey, model string) (bool, error) {
	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"parts": []map[string]string{{"text": "Say test"}}},
		},
	}

	body, _ := json.Marshal(reqBody)

	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", baseURL, model, apiKey)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return false, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := v.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}