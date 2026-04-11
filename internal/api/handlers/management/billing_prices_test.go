package management

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/usage"
)

func TestBillingPricesHandlersRoundTrip(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dbPath := filepath.Join(t.TempDir(), "usage.sqlite")
	t.Setenv("USAGE_SQLITE_PATH", dbPath)

	handler := &Handler{}
	router := gin.New()
	router.GET("/billing-prices", handler.GetBillingPrices)
	router.PUT("/billing-prices", handler.PutBillingPrices)

	getBefore := httptest.NewRecorder()
	router.ServeHTTP(getBefore, httptest.NewRequest(http.MethodGet, "/billing-prices", nil))
	if getBefore.Code != http.StatusOK {
		t.Fatalf("initial GET status = %d, want %d", getBefore.Code, http.StatusOK)
	}
	var before billingPricesPayload
	if err := json.Unmarshal(getBefore.Body.Bytes(), &before); err != nil {
		t.Fatalf("unmarshal initial GET response: %v", err)
	}
	if len(before.Prices) != 0 {
		t.Fatalf("initial prices len = %d, want 0", len(before.Prices))
	}

	putBody := billingPricesPayload{
		Prices: []usage.BillingModelPrice{
			{
				ModelName:          " gpt-5.4 ",
				InputPricePerM:     2.5,
				OutputPricePerM:    10,
				ReasoningPricePerM: 1.5,
				CachedPricePerM:    0.3,
			},
			{
				ModelName:       "claude-sonnet-4",
				InputPricePerM:  3,
				OutputPricePerM: 15,
			},
			{
				ModelName: "",
			},
		},
	}
	putData, err := json.Marshal(putBody)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	putReq := httptest.NewRequest(http.MethodPut, "/billing-prices", bytes.NewReader(putData))
	putReq.Header.Set("Content-Type", "application/json")
	putResp := httptest.NewRecorder()
	router.ServeHTTP(putResp, putReq)
	if putResp.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, want %d, body=%s", putResp.Code, http.StatusOK, putResp.Body.String())
	}

	var putResult billingPricesPayload
	if err := json.Unmarshal(putResp.Body.Bytes(), &putResult); err != nil {
		t.Fatalf("unmarshal PUT response: %v", err)
	}
	if len(putResult.Prices) != 2 {
		t.Fatalf("PUT returned %d prices, want 2", len(putResult.Prices))
	}
	if putResult.Prices[0].ModelName != "gpt-5.4" {
		t.Fatalf("trimmed model name = %q, want %q", putResult.Prices[0].ModelName, "gpt-5.4")
	}

	getAfter := httptest.NewRecorder()
	router.ServeHTTP(getAfter, httptest.NewRequest(http.MethodGet, "/billing-prices", nil))
	if getAfter.Code != http.StatusOK {
		t.Fatalf("GET after PUT status = %d, want %d", getAfter.Code, http.StatusOK)
	}

	var after billingPricesPayload
	if err := json.Unmarshal(getAfter.Body.Bytes(), &after); err != nil {
		t.Fatalf("unmarshal GET after PUT response: %v", err)
	}
	if len(after.Prices) != 2 {
		t.Fatalf("persisted prices len = %d, want 2", len(after.Prices))
	}
	if after.Prices[0].ModelName != "claude-sonnet-4" || after.Prices[1].ModelName != "gpt-5.4" {
		t.Fatalf("unexpected persisted ordering/content: %#v", after.Prices)
	}
	if after.Prices[1].UpdatedAt == "" {
		t.Fatalf("expected persisted UpdatedAt to be set")
	}
}

func TestPutBillingPricesRejectsInvalidPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dbPath := filepath.Join(t.TempDir(), "usage.sqlite")
	t.Setenv("USAGE_SQLITE_PATH", dbPath)

	handler := &Handler{}
	router := gin.New()
	router.PUT("/billing-prices", handler.PutBillingPrices)

	t.Run("duplicate model name", func(t *testing.T) {
		body := billingPricesPayload{
			Prices: []usage.BillingModelPrice{
				{ModelName: "gpt-5.4", InputPricePerM: 1},
				{ModelName: " GPT-5.4 ", OutputPricePerM: 2},
			},
		}
		assertBillingPricesPutError(t, router, body, http.StatusBadRequest, "duplicate model_name")
	})

	t.Run("negative price", func(t *testing.T) {
		body := billingPricesPayload{
			Prices: []usage.BillingModelPrice{
				{ModelName: "gpt-5.4", InputPricePerM: -1},
			},
		}
		assertBillingPricesPutError(t, router, body, http.StatusBadRequest, "price values must be >= 0")
	})

	t.Run("invalid json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/billing-prices", bytes.NewBufferString(`{`))
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		if resp.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d, body=%s", resp.Code, http.StatusBadRequest, resp.Body.String())
		}
		var payload map[string]string
		if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if payload["error"] != "invalid body" {
			t.Fatalf("error = %q, want %q", payload["error"], "invalid body")
		}
	})
}

func assertBillingPricesPutError(t *testing.T, router *gin.Engine, body billingPricesPayload, wantStatus int, wantError string) {
	t.Helper()

	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/billing-prices", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != wantStatus {
		t.Fatalf("status = %d, want %d, body=%s", resp.Code, wantStatus, resp.Body.String())
	}
	var payload map[string]string
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload["error"] != wantError {
		t.Fatalf("error = %q, want %q", payload["error"], wantError)
	}
}
