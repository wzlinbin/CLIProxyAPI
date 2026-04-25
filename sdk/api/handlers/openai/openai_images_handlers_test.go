package openai

import (
	"context"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/interfaces"
	"github.com/tidwall/gjson"
)

func TestCollectImagesFromResponsesStreamUsesOutputItemDoneFallback(t *testing.T) {
	data := make(chan []byte, 2)
	errs := make(chan *interfaces.ErrorMessage)
	data <- []byte(`data: {"type":"response.output_item.done","item":{"type":"image_generation_call","output_format":"png","result":"aGVsbG8=","revised_prompt":"ok"}}`)
	data <- []byte(`data: {"type":"response.completed","response":{"created_at":1700000000,"output":[]}}`)
	close(data)
	close(errs)

	out, errMsg := collectImagesFromResponsesStream(context.Background(), data, errs, "b64_json")
	if errMsg != nil {
		t.Fatalf("unexpected error: %v", errMsg.Error)
	}
	if got := gjson.GetBytes(out, "data.0.b64_json").String(); got != "aGVsbG8=" {
		t.Fatalf("unexpected b64_json: %q", got)
	}
	if got := gjson.GetBytes(out, "data.0.revised_prompt").String(); got != "ok" {
		t.Fatalf("unexpected revised_prompt: %q", got)
	}
	if got := gjson.GetBytes(out, "output_format").String(); got != "png" {
		t.Fatalf("unexpected output_format: %q", got)
	}
}

func TestExtractImagesFromResponsesCompletedKeepsOutputResults(t *testing.T) {
	payload := []byte(`{"type":"response.completed","response":{"created_at":1700000000,"output":[{"type":"image_generation_call","output_format":"webp","result":"Ymll"}]}}`)

	results, createdAt, _, firstMeta, err := extractImagesFromResponsesCompleted(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if createdAt != 1700000000 {
		t.Fatalf("unexpected createdAt: %d", createdAt)
	}
	if len(results) != 1 || results[0].Result != "Ymll" {
		t.Fatalf("unexpected results: %+v", results)
	}
	if firstMeta.OutputFormat != "webp" {
		t.Fatalf("unexpected first meta: %+v", firstMeta)
	}
}
