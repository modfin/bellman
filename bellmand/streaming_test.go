package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/modfin/bellman"
	"github.com/modfin/bellman/models"
	"github.com/modfin/bellman/models/gen"
	"github.com/modfin/bellman/prompt"
	"github.com/prometheus/client_golang/prometheus"
)

// Mock generator that implements the required interface
type mockGen struct {
	provider string
}

func (m *mockGen) Provider() string {
	return m.provider
}

func (m *mockGen) Generator(options ...gen.Option) *gen.Generator {
	gen := &gen.Generator{
		Prompter: &mockPrompter{},
		Request:  gen.Request{},
	}
	for _, op := range options {
		gen = op(gen)
	}
	return gen
}

// Mock prompter that implements the required interface
type mockPrompter struct {
	request gen.Request
}

func (m *mockPrompter) SetRequest(request gen.Request) {
	m.request = request
}

func (m *mockPrompter) Prompt(conversation ...prompt.Prompt) (*gen.Response, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockPrompter) Stream(conversation ...prompt.Prompt) (<-chan *gen.StreamResponse, error) {
	stream := make(chan *gen.StreamResponse, 10)

	go func() {
		defer close(stream)

		// Send a few streaming responses
		stream <- &gen.StreamResponse{
			Type:    gen.TYPE_DELTA,
			Role:    prompt.AssistantRole,
			Content: "Hello",
		}

		stream <- &gen.StreamResponse{
			Type:    gen.TYPE_DELTA,
			Role:    prompt.AssistantRole,
			Content: " world",
		}

		stream <- &gen.StreamResponse{
			Type: gen.TYPE_METADATA,
			Metadata: &models.Metadata{
				Model:        "test-model",
				InputTokens:  10,
				OutputTokens: 5,
				TotalTokens:  15,
			},
		}

		stream <- &gen.StreamResponse{
			Type: gen.TYPE_EOF,
		}
	}()

	return stream, nil
}

func TestStreamingEndpoint_Basic(t *testing.T) {
	// Initialize logger for testing
	logger = slog.Default()

	// Reset prometheus registry to avoid duplicate registration
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	// Create a test configuration
	cfg := Config{
		ApiKeys:  []string{"test-key"},
		HttpPort: 8080,
	}

	// Create a real proxy and register a mock generator
	proxy := bellman.NewProxy()
	mockGen := &mockGen{provider: "test"}
	proxy.RegisterGen(mockGen)

	// Create the router
	r := chi.NewRouter()
	r.Use(auth(cfg))
	r.Route("/gen", Gen(proxy, cfg))

	// Create test request
	request := gen.FullRequest{
		Request: gen.Request{
			Model: gen.Model{Provider: "test", Name: "test-model"},
		},
		Prompts: []prompt.Prompt{
			{Role: prompt.UserRole, Text: "Hello, how are you?"},
		},
	}

	body, _ := json.Marshal(request)

	// Create HTTP request
	req := httptest.NewRequest("POST", "/gen/stream", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-key_test-key")

	// Create response recorder
	w := httptest.NewRecorder()

	// Serve the request
	r.ServeHTTP(w, req)

	// Check response status
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Check SSE headers
	if w.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("expected Content-Type text/event-stream, got %s", w.Header().Get("Content-Type"))
	}

	if w.Header().Get("Cache-Control") != "no-cache" {
		t.Errorf("expected Cache-Control no-cache, got %s", w.Header().Get("Cache-Control"))
	}

	if w.Header().Get("Connection") != "keep-alive" {
		t.Errorf("expected Connection keep-alive, got %s", w.Header().Get("Connection"))
	}

	// Check response body contains SSE data
	bodyStr := w.Body.String()
	if !strings.Contains(bodyStr, "data: ") {
		t.Errorf("expected SSE data format, got: %s", bodyStr)
	}

	// Check that we have multiple SSE events
	lines := strings.Split(bodyStr, "\n")
	dataLines := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "data: ") {
			dataLines++
		}
	}

	if dataLines < 2 {
		t.Errorf("expected at least 2 SSE data events, got %d", dataLines)
	}

	// Verify the content of the SSE events
	if !strings.Contains(bodyStr, `"type":"delta"`) {
		t.Errorf("expected delta type in SSE data")
	}

	if !strings.Contains(bodyStr, `"type":"metadata"`) {
		t.Errorf("expected metadata type in SSE data")
	}

	if !strings.Contains(bodyStr, `"type":"EOF"`) {
		t.Errorf("expected EOF type in SSE data")
	}
}

func TestStreamingEndpoint_Authentication(t *testing.T) {
	// Initialize logger for testing
	logger = slog.Default()

	// Reset prometheus registry to avoid duplicate registration
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	// Create a test configuration
	cfg := Config{
		ApiKeys:  []string{"test-key"},
		HttpPort: 8080,
	}

	// Create a real proxy
	proxy := bellman.NewProxy()

	// Create the router
	r := chi.NewRouter()
	r.Use(auth(cfg))
	r.Route("/gen", Gen(proxy, cfg))

	// Create test request without authentication
	request := gen.FullRequest{
		Request: gen.Request{
			Model: gen.Model{Provider: "test", Name: "test-model"},
		},
		Prompts: []prompt.Prompt{
			{Role: prompt.UserRole, Text: "Hello"},
		},
	}

	body, _ := json.Marshal(request)
	req := httptest.NewRequest("POST", "/gen/stream", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// No Authorization header

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should return 401 Unauthorized
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestStreamingEndpoint_InvalidRequest(t *testing.T) {
	// Initialize logger for testing
	logger = slog.Default()

	// Reset prometheus registry to avoid duplicate registration
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	// Create a test configuration
	cfg := Config{
		ApiKeys:  []string{"test-key"},
		HttpPort: 8080,
	}

	// Create a real proxy
	proxy := bellman.NewProxy()

	// Create the router
	r := chi.NewRouter()
	r.Use(auth(cfg))
	r.Route("/gen", Gen(proxy, cfg))

	// Create invalid JSON request
	req := httptest.NewRequest("POST", "/gen/stream", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-key_test-key")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should return 400 Bad Request
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestStreamingEndpoint_ProviderNotFound(t *testing.T) {
	// Initialize logger for testing
	logger = slog.Default()

	// Reset prometheus registry to avoid duplicate registration
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	// Create a test configuration
	cfg := Config{
		ApiKeys:  []string{"test-key"},
		HttpPort: 8080,
	}

	// Create a real proxy without registering any providers
	proxy := bellman.NewProxy()

	// Create the router
	r := chi.NewRouter()
	r.Use(auth(cfg))
	r.Route("/gen", Gen(proxy, cfg))

	// Create test request with unknown provider
	request := gen.FullRequest{
		Request: gen.Request{
			Model: gen.Model{Provider: "unknown", Name: "test-model"},
		},
		Prompts: []prompt.Prompt{
			{Role: prompt.UserRole, Text: "Hello"},
		},
	}

	body, _ := json.Marshal(request)
	req := httptest.NewRequest("POST", "/gen/stream", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-key_test-key")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should return 500 Internal Server Error
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}

	// Check error message
	bodyStr := w.Body.String()
	if !strings.Contains(bodyStr, "could not get generator") {
		t.Errorf("expected error about generator, got: %s", bodyStr)
	}
}
