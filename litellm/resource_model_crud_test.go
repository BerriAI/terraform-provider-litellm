package litellm

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// notFoundBody returns a JSON body that triggers isModelNotFoundError via the detail.error path.
func notFoundBody() []byte {
	return []byte(`{"detail": {"error": "Model id = test-id not found on litellm proxy"}}`)
}

// newTestModelResourceData creates *schema.ResourceData for the model resource,
// pre-populated with required fields and the given ID.
func newTestModelResourceData(t *testing.T, id string) *schema.ResourceData {
	t.Helper()
	d := schema.TestResourceDataRaw(t, resourceLiteLLMModel().Schema, map[string]interface{}{
		"model_name":                         "test-model",
		"custom_llm_provider":                "vertex_ai",
		"base_model":                         "claude-sonnet-4-5",
		"tier":                               "enterprise",
		"mode":                               "chat",
		"tpm":                                0,
		"rpm":                                0,
		"thinking_enabled":                   false,
		"thinking_budget_tokens":             1024,
		"merge_reasoning_content_in_choices": false,
	})
	d.SetId(id)
	return d
}

// successBody returns a JSON body that handleAPIResponse can parse into a ModelResponse.
func successBody(id string) []byte {
	resp := ModelResponse{
		ModelName: "test-model",
		LiteLLMParams: LiteLLMParams{
			CustomLLMProvider: "vertex_ai",
			Model:             "vertex_ai/claude-sonnet-4-5",
		},
		ModelInfo: ModelInfo{
			ID:        id,
			BaseModel: "claude-sonnet-4-5",
			Tier:      "enterprise",
		},
	}
	b, _ := json.Marshal(resp)
	return b
}

func TestRetryModelRead_SuccessOnFirstAttempt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(successBody("test-id"))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key", true)
	d := newTestModelResourceData(t, "test-id")

	err := retryModelRead(d, client, 10)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if d.Id() != "test-id" {
		t.Fatalf("expected ID 'test-id', got %q", d.Id())
	}
}

// TestRetryModelRead_SuccessAfterRetries simulates the core race condition:
// the model is not yet visible on the first reads (cache miss → 404),
// then becomes visible on the 4th attempt.
func TestRetryModelRead_SuccessAfterRetries(t *testing.T) {
	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		if n <= 3 {
			w.WriteHeader(http.StatusBadRequest)
			w.Write(notFoundBody())
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(successBody("test-id"))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key", true)
	d := newTestModelResourceData(t, "test-id")

	err := retryModelRead(d, client, 10)
	if err != nil {
		t.Fatalf("expected nil error after eventual success, got: %v", err)
	}
	if d.Id() != "test-id" {
		t.Fatalf("expected ID 'test-id', got %q", d.Id())
	}
	if atomic.LoadInt32(&callCount) != 4 {
		t.Fatalf("expected 4 HTTP calls (3 misses + 1 hit), got %d", callCount)
	}
}

// TestRetryModelRead_ExhaustsRetries verifies that after all attempts the function
// returns an error describing the exhaustion (not "model_not_found") and preserves the ID.
func TestRetryModelRead_ExhaustsRetries(t *testing.T) {
	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusBadRequest)
		w.Write(notFoundBody())
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key", true)
	d := newTestModelResourceData(t, "test-id")

	err := retryModelRead(d, client, 3)
	if err == nil {
		t.Fatal("expected error after exhausting retries, got nil")
	}
	if atomic.LoadInt32(&callCount) != 3 {
		t.Fatalf("expected exactly 3 HTTP calls, got %d", callCount)
	}
	// ID must still be set so Terraform can retry on next apply
	if d.Id() != "test-id" {
		t.Fatalf("expected ID to be preserved as 'test-id', got %q", d.Id())
	}
}

// TestRetryModelRead_IDRestoredBetweenRetries verifies that resourceLiteLLMModelRead
// clears the ID on a 404 and that retryModelRead restores it before the next attempt,
// allowing the URL in the subsequent GET to still carry the correct model ID.
func TestRetryModelRead_IDRestoredBetweenRetries(t *testing.T) {
	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		if n == 1 {
			w.WriteHeader(http.StatusBadRequest)
			w.Write(notFoundBody())
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(successBody("my-model-id"))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key", true)
	d := newTestModelResourceData(t, "my-model-id")

	err := retryModelRead(d, client, 2)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if d.Id() != "my-model-id" {
		t.Fatalf("expected ID 'my-model-id', got %q", d.Id())
	}
}

// TestRetryModelRead_ServerError verifies that a 500 response is eventually
// surfaced as an error after all retries are exhausted and that the error
// message is not "model_not_found" (to distinguish it from cache-miss errors).
func TestRetryModelRead_ServerError(t *testing.T) {
	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": {"message": "internal server error"}}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key", true)
	d := newTestModelResourceData(t, "test-id")

	const maxRetries = 2
	err := retryModelRead(d, client, maxRetries)
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
	// retryModelRead retries on all errors; verify it exhausted the full count.
	if atomic.LoadInt32(&callCount) != maxRetries {
		t.Fatalf("expected %d HTTP calls, got %d", maxRetries, callCount)
	}
	// The error must not look like a model_not_found (which would clear state).
	if err.Error() == "model_not_found" {
		t.Fatal("500 error must not be reported as model_not_found")
	}
}

// TestRetryModelRead_ConnectionError verifies that a connection-level failure
// (server closed) is also surfaced without looping indefinitely.
func TestRetryModelRead_ConnectionError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close() // closed immediately

	client := NewClient(srv.URL, "test-key", true)
	d := newTestModelResourceData(t, "test-id")

	err := retryModelRead(d, client, 3)
	if err == nil {
		t.Fatal("expected error for connection failure, got nil")
	}
	fmt.Printf("connection error (expected): %v\n", err)
}
