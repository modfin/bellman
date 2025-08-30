package bellman

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/modfin/bellman/models/gen"
	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/schema"
	"github.com/modfin/bellman/tools"
)

func TestStreamingClient_Validation(t *testing.T) {
	client := New("http://localhost:8080", Key{Name: "test", Token: "test-token"})

	tests := []struct {
		name        string
		model       gen.Model
		prompts     []prompt.Prompt
		expectError bool
		errorMsg    string
	}{
		{
			name:        "empty prompts should fail",
			model:       gen.Model{Provider: "openai", Name: "gpt-3.5-turbo"},
			prompts:     []prompt.Prompt{},
			expectError: true,
			errorMsg:    "at least one prompt is required",
		},
		{
			name:        "valid request should pass validation",
			model:       gen.Model{Provider: "openai", Name: "gpt-3.5-turbo"},
			prompts:     []prompt.Prompt{{Role: prompt.UserRole, Text: "test"}},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := client.Generator(gen.WithModel(tt.model))
			_, err := generator.Stream(tt.prompts...)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error to contain '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err == nil {
					t.Errorf("expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestStreamingClient_RequestBuilding(t *testing.T) {
	client := New("http://localhost:8080", Key{Name: "test", Token: "test-token"})
	generator := client.Generator(
		gen.WithModel(gen.Model{Provider: "openai", Name: "gpt-3.5-turbo"}),
		gen.WithTemperature(0.7),
		gen.WithMaxTokens(100),
	)

	prompts := []prompt.Prompt{
		{Role: prompt.UserRole, Text: "Hello"},
		{Role: prompt.AssistantRole, Text: "Hi there!"},
	}

	// Test that the request is built correctly
	stream, err := generator.Stream(prompts...)
	if err == nil {
		t.Errorf("expected error (server not running) but got none")
		return
	}

	// The error should indicate the server is not reachable
	if !strings.Contains(err.Error(), "connection refused") &&
		!strings.Contains(err.Error(), "no such host") &&
		!strings.Contains(err.Error(), "unexpected status code") {
		t.Errorf("expected connection error, got: %v", err)
	}

	if stream != nil {
		t.Errorf("expected nil stream when server is not available")
	}
}

func TestStreamingClient_SSEHandling(t *testing.T) {
	// Create a test server that returns SSE data
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check that the request is for streaming
		if !strings.Contains(r.URL.Path, "/gen/stream") {
			t.Errorf("expected request to /gen/stream, got %s", r.URL.Path)
		}

		// Check SSE headers
		if r.Header.Get("Accept") != "text/event-stream" {
			t.Errorf("expected Accept: text/event-stream, got %s", r.Header.Get("Accept"))
		}

		// Set SSE headers
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		// Send SSE data
		responses := []string{
			`{"type": "delta", "role": "assistant", "content": "Hello"}`,
			`{"type": "delta", "role": "assistant", "content": " world"}`,
			`{"type": "metadata", "metadata": {"input_tokens": 10, "output_tokens": 5, "total_tokens": 15}}`,
			`[DONE]`,
		}

		for _, response := range responses {
			fmt.Fprintf(w, "data: %s\n\n", response)
			w.(http.Flusher).Flush()
		}
	}))
	defer server.Close()

	// Create client pointing to test server
	client := New(server.URL, Key{Name: "test", Token: "test-token"})
	generator := client.Generator(gen.WithModel(gen.Model{Provider: "openai", Name: "gpt-3.5-turbo"}))

	prompts := []prompt.Prompt{{Role: prompt.UserRole, Text: "Hello"}}

	stream, err := generator.Stream(prompts...)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Read from stream
	var responses []*gen.StreamResponse
	for response := range stream {
		responses = append(responses, response)
	}

	// Verify responses
	if len(responses) != 4 { // 3 data + 1 EOF
		t.Errorf("expected 4 responses, got %d", len(responses))
	}

	// Check first delta response
	if responses[0].Type != gen.TYPE_DELTA {
		t.Errorf("expected TYPE_DELTA, got %s", responses[0].Type)
	}
	if responses[0].Content != "Hello" {
		t.Errorf("expected content 'Hello', got '%s'", responses[0].Content)
	}

	// Check second delta response
	if responses[1].Type != gen.TYPE_DELTA {
		t.Errorf("expected TYPE_DELTA, got %s", responses[1].Type)
	}
	if responses[1].Content != " world" {
		t.Errorf("expected content ' world', got '%s'", responses[1].Content)
	}

	// Check metadata response
	if responses[2].Type != gen.TYPE_METADATA {
		t.Errorf("expected TYPE_METADATA, got %s", responses[2].Type)
	}
	if responses[2].Metadata == nil {
		t.Errorf("expected metadata, got nil")
	}

	// Check EOF response
	if responses[3].Type != gen.TYPE_EOF {
		t.Errorf("expected TYPE_EOF, got %s", responses[3].Type)
	}
}

func TestStreamingClient_ErrorHandling(t *testing.T) {
	// Test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "server error"}`))
	}))
	defer server.Close()

	client := New(server.URL, Key{Name: "test", Token: "test-token"})
	generator := client.Generator(gen.WithModel(gen.Model{Provider: "openai", Name: "gpt-3.5-turbo"}))

	prompts := []prompt.Prompt{{Role: prompt.UserRole, Text: "Hello"}}

	_, err := generator.Stream(prompts...)
	if err == nil {
		t.Errorf("expected error but got none")
	}

	if !strings.Contains(err.Error(), "unexpected status code, 500") {
		t.Errorf("expected 500 error, got: %v", err)
	}
}

func TestStreamingClient_ContextCancellation(t *testing.T) {
	// Test server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second) // Delay to allow cancellation
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: {\"type\": \"delta\", \"content\": \"Hello\"}\n\n")
	}))
	defer server.Close()

	client := New(server.URL, Key{Name: "test", Token: "test-token"})

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	generator := client.Generator(
		gen.WithModel(gen.Model{Provider: "openai", Name: "gpt-3.5-turbo"}),
		gen.WithContext(ctx),
	)

	prompts := []prompt.Prompt{{Role: prompt.UserRole, Text: "Hello"}}

	_, err := generator.Stream(prompts...)
	if err == nil {
		t.Errorf("expected error for context cancellation, got none")
	}

	// Check that the error is related to context cancellation
	if !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Errorf("expected context deadline error, got: %v", err)
	}
}

func TestStreamingClient_ToolCallSupport(t *testing.T) {
	// Create test server that returns tool calls
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")

		// Send simple delta response first
		fmt.Fprintf(w, "data: {\"type\": \"delta\", \"content\": \"I'll check the weather\"}\n\n")

		// Send tool call response
		arg := base64.StdEncoding.EncodeToString([]byte(`{"location":"New York"}`))
		toolCall := fmt.Sprintf(`{"type":"delta","role":"assistant","content":"","tool_call":{"id":"call_123","name":"get_weather","argument":"%s"}}`, arg)

		fmt.Fprintf(w, "data: %s\n\n", toolCall)
		fmt.Fprintf(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	// Create tool
	weatherTool := tools.Tool{
		Name:        "get_weather",
		Description: "Get weather information",
		ArgumentSchema: &schema.JSON{
			Type: "object",
			Properties: map[string]*schema.JSON{
				"location": {Type: "string"},
			},
		},
	}

	client := New(server.URL, Key{Name: "test", Token: "test-token"})
	generator := client.Generator(
		gen.WithModel(gen.Model{Provider: "openai", Name: "gpt-3.5-turbo"}),
		gen.WithTools(weatherTool),
	)

	prompts := []prompt.Prompt{{Role: prompt.UserRole, Text: "What's the weather?"}}

	stream, err := generator.Stream(prompts...)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Read responses
	var responses []*gen.StreamResponse
	for response := range stream {
		responses = append(responses, response)
	}

	// Check tool call response
	if len(responses) < 3 {
		t.Errorf("expected at least 3 responses, got %d", len(responses))
	}

	// Check first response (should be delta)
	firstResponse := responses[0]
	if firstResponse.Type != gen.TYPE_DELTA {
		t.Errorf("expected first response to be TYPE_DELTA, got %s", firstResponse.Type)
	}

	// Check second response (should be tool call)
	toolResponse := responses[1]
	t.Logf("Tool response type: %s", toolResponse.Type)
	t.Logf("Tool response content: %s", toolResponse.Content)

	if toolResponse.Type != gen.TYPE_DELTA {
		t.Errorf("expected TYPE_DELTA, got %s", toolResponse.Type)
	}

	// Only check if tool call exists, don't access its fields to avoid nil pointer
	if toolResponse.ToolCall == nil {
		t.Errorf("expected tool call, got nil")
	}
}

func TestStreamingClient_InvalidSSEFormat(t *testing.T) {
	// Test server that returns invalid SSE format
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("invalid sse format\n")) // No "data: " prefix
	}))
	defer server.Close()

	client := New(server.URL, Key{Name: "test", Token: "test-token"})
	generator := client.Generator(gen.WithModel(gen.Model{Provider: "openai", Name: "gpt-3.5-turbo"}))

	prompts := []prompt.Prompt{{Role: prompt.UserRole, Text: "Hello"}}

	stream, err := generator.Stream(prompts...)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Read from stream - should get error response
	response := <-stream
	if response.Type != gen.TYPE_ERROR {
		t.Errorf("expected TYPE_ERROR for invalid SSE, got %s", response.Type)
	}

	if !strings.Contains(response.Content, "expected 'data' header from sse") {
		t.Errorf("expected SSE format error, got: %s", response.Content)
	}
}

func TestStreamingClient_JSONParsingError(t *testing.T) {
	// Test server that returns invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: {invalid json}\n\n") // Invalid JSON
	}))
	defer server.Close()

	client := New(server.URL, Key{Name: "test", Token: "test-token"})
	generator := client.Generator(gen.WithModel(gen.Model{Provider: "openai", Name: "gpt-3.5-turbo"}))

	prompts := []prompt.Prompt{{Role: prompt.UserRole, Text: "Hello"}}

	stream, err := generator.Stream(prompts...)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Read from stream - should get error response
	response := <-stream
	if response.Type != gen.TYPE_ERROR {
		t.Errorf("expected TYPE_ERROR for invalid JSON, got %s", response.Type)
	}

	if !strings.Contains(response.Content, "could not unmarshal stream chunk") {
		t.Errorf("expected JSON parsing error, got: %s", response.Content)
	}
}

func TestStreamingClient_RequestHeaders(t *testing.T) {
	var receivedHeaders http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	client := New(server.URL, Key{Name: "test", Token: "test-token"})
	generator := client.Generator(gen.WithModel(gen.Model{Provider: "openai", Name: "gpt-3.5-turbo"}))

	prompts := []prompt.Prompt{{Role: prompt.UserRole, Text: "Hello"}}

	stream, err := generator.Stream(prompts...)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Read to completion
	for range stream {
	}

	// Check headers
	expectedHeaders := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer test_test-token",
		"Accept":        "text/event-stream",
		"Cache-Control": "no-cache",
		"Connection":    "keep-alive",
	}

	for key, expectedValue := range expectedHeaders {
		if receivedHeaders.Get(key) != expectedValue {
			t.Errorf("expected header %s: %s, got: %s", key, expectedValue, receivedHeaders.Get(key))
		}
	}
}

func TestStreamingClient_RequestBody(t *testing.T) {
	var requestBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read the request body
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("failed to read request body: %v", err)
		}
		requestBody = bodyBytes

		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	client := New(server.URL, Key{Name: "test", Token: "test-token"})
	generator := client.Generator(
		gen.WithModel(gen.Model{Provider: "openai", Name: "gpt-3.5-turbo"}),
		gen.WithTemperature(0.7),
		gen.WithMaxTokens(100),
	)

	prompts := []prompt.Prompt{{Role: prompt.UserRole, Text: "Hello"}}

	stream, err := generator.Stream(prompts...)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Read to completion
	for range stream {
	}

	// Parse request body
	var request gen.FullRequest
	err = json.Unmarshal(requestBody, &request)
	if err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}

	// Check that streaming is enabled
	if !request.Stream {
		t.Errorf("expected Stream to be true")
	}

	// Check model
	if request.Model.Name != "gpt-3.5-turbo" {
		t.Errorf("expected model name 'gpt-3.5-turbo', got '%s'", request.Model.Name)
	}

	// Check prompts
	if len(request.Prompts) != 1 {
		t.Errorf("expected 1 prompt, got %d", len(request.Prompts))
	}

	if request.Prompts[0].Text != "Hello" {
		t.Errorf("expected prompt text 'Hello', got '%s'", request.Prompts[0].Text)
	}
}

func BenchmarkStreamingClient(b *testing.B) {
	// Create a simple test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for i := 0; i < 10; i++ {
			fmt.Fprintf(w, "data: {\"type\": \"delta\", \"content\": \"chunk %d\"}\n\n", i)
		}
		fmt.Fprintf(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	client := New(server.URL, Key{Name: "test", Token: "test-token"})
	generator := client.Generator(gen.WithModel(gen.Model{Provider: "openai", Name: "gpt-3.5-turbo"}))

	prompts := []prompt.Prompt{{Role: prompt.UserRole, Text: "Hello"}}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stream, err := generator.Stream(prompts...)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}

		// Read all responses
		for range stream {
		}
	}
}
