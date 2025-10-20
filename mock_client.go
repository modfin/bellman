package bellman

import (
	"context"
	"fmt"
	"github.com/modfin/bellman/models"
	"github.com/modfin/bellman/models/embed"
	"github.com/modfin/bellman/models/gen"
	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/tools"
	"log/slog"
	"math/rand"
	"strings"
	"time"
)

const MockProvider = "Mock"

type MockClient struct {
	Log *slog.Logger `json:"-"`
}

func NewMock() *MockClient {
	return &MockClient{}
}

func (m *MockClient) Provider() string {
	return MockProvider
}

func (m *MockClient) log(msg string, args ...any) {
	if m.Log == nil {
		return
	}
	m.Log.Debug("[bellman/mock] "+msg, args...)
}

func (m *MockClient) SetLogger(logger *slog.Logger) *MockClient {
	m.Log = logger
	return m
}

// Embed interface implementation

func (m *MockClient) Embed(request *embed.Request) (*embed.Response, error) {
	m.log("[embed] request", "model", request.Model.FQN(), "texts", len(request.Texts))

	// Generate mock embeddings (384 dimensions for simplicity)
	embeddings := make([][]float64, len(request.Texts))
	dimensions := 384

	for i, text := range request.Texts {
		embeddings[i] = make([]float64, dimensions)
		// Create deterministic embeddings based on text content
		seed := hashString(text)
		rng := rand.New(rand.NewSource(seed))
		for j := 0; j < dimensions; j++ {
			embeddings[i][j] = rng.Float64()*2 - 1 // Values between -1 and 1
		}
	}

	// Calculate mock token count
	totalTokens := 0
	for _, text := range request.Texts {
		// Rough approximation: 4 characters per token
		totalTokens += len(text) / 4
	}

	response := &embed.Response{
		Embeddings: embeddings,
		Metadata: models.Metadata{
			Model:       request.Model.FQN(),
			TotalTokens: totalTokens,
		},
	}

	m.log("[embed] response", "model", request.Model.FQN(), "token-total", response.Metadata.TotalTokens)

	return response, nil
}

func (m *MockClient) EmbedDocument(request *embed.DocumentRequest) (*embed.DocumentResponse, error) {
	m.log("[embed] document request", "model", request.Model.FQN(), "chunks", len(request.DocumentChunks))

	// Generate mock embeddings (768 dimensions for document embeddings)
	embeddings := make([][]float64, len(request.DocumentChunks))
	dimensions := 768

	for i, chunk := range request.DocumentChunks {
		embeddings[i] = make([]float64, dimensions)
		// Create deterministic embeddings based on chunk content
		seed := hashString(chunk)
		rng := rand.New(rand.NewSource(seed))
		for j := 0; j < dimensions; j++ {
			embeddings[i][j] = rng.Float64()*2 - 1 // Values between -1 and 1
		}
	}

	// Calculate mock token count
	totalTokens := 0
	for _, chunk := range request.DocumentChunks {
		// Rough approximation: 4 characters per token
		totalTokens += len(chunk) / 4
	}

	response := &embed.DocumentResponse{
		Embeddings: embeddings,
		Metadata: models.Metadata{
			Model:       request.Model.FQN(),
			TotalTokens: totalTokens,
		},
	}

	m.log("[embed] document response", "model", request.Model.FQN(), "token-total", response.Metadata.TotalTokens)

	return response, nil
}

// Gen interface implementation

func (m *MockClient) Generator(options ...gen.Option) *gen.Generator {
	g := &gen.Generator{
		Prompter: &mockGenerator{
			mock: m,
		},
		Request: gen.Request{},
	}
	for _, op := range options {
		g = op(g)
	}
	return g
}

type mockGenerator struct {
	mock    *MockClient
	request gen.Request
}

func (g *mockGenerator) SetRequest(request gen.Request) {
	g.request = request
}

func (g *mockGenerator) Prompt(conversation ...prompt.Prompt) (*gen.Response, error) {
	g.mock.log("[gen] request", "model", g.request.Model.FQN(), "prompts", len(conversation))

	// Build a mock response based on the conversation
	var responseText strings.Builder
	responseText.WriteString("This is a mock response from the ")
	responseText.WriteString(g.request.Model.FQN())
	responseText.WriteString(" model.\n\n")

	// Echo back the user messages
	for _, p := range conversation {
		switch p.Role {
		case prompt.UserRole:
			responseText.WriteString("You asked: ")
			responseText.WriteString(p.Text)
			responseText.WriteString("\n")
		}
	}

	responseText.WriteString("\nMock answer: This is simulated AI response content. ")
	responseText.WriteString("The actual implementation would call a real AI model here.")

	// Calculate mock token counts
	inputTokens := 0
	for _, p := range conversation {
		inputTokens += len(p.Text) / 4 // Rough approximation
	}
	outputTokens := len(responseText.String()) / 4

	response := &gen.Response{
		Texts: []string{responseText.String()},
		Metadata: models.Metadata{
			Model:        g.request.Model.FQN(),
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
			TotalTokens:  inputTokens + outputTokens,
		},
		Tools: []tools.Call{},
	}

	g.mock.log("[gen] response",
		"model", g.request.Model.FQN(),
		"token-input", response.Metadata.InputTokens,
		"token-output", response.Metadata.OutputTokens,
		"token-total", response.Metadata.TotalTokens,
	)

	return response, nil
}

func (g *mockGenerator) Stream(conversation ...prompt.Prompt) (<-chan *gen.StreamResponse, error) {
	g.mock.log("[gen] stream request", "model", g.request.Model.FQN(), "prompts", len(conversation))

	stream := make(chan *gen.StreamResponse, 100)

	go func() {
		defer close(stream)

		ctx := g.request.Context
		if ctx == nil {
			ctx = context.Background()
		}

		// Build mock response text
		mockText := fmt.Sprintf("This is a streaming mock response from %s. ", g.request.Model.FQN())

		// Echo back user messages
		for _, p := range conversation {
			if p.Role == prompt.UserRole {
				mockText += fmt.Sprintf("You asked: %s. ", p.Text[:min(50, len(p.Text))])
			}
		}

		mockText += "Streaming simulated content word by word..."

		// Stream the response word by word
		words := strings.Fields(mockText)
		for i, word := range words {
			select {
			case <-ctx.Done():
				stream <- &gen.StreamResponse{
					Type:    gen.TYPE_ERROR,
					Content: "stream cancelled",
				}
				return
			default:
			}

			// Add space after each word except the last one
			content := word
			if i < len(words)-1 {
				content += " "
			}

			stream <- &gen.StreamResponse{
				Type:     gen.TYPE_DELTA,
				Role:     prompt.AssistantRole,
				Index:    0,
				Content:  content,
				ToolCall: nil,
				Metadata: nil,
			}

			// Simulate streaming delay
			time.Sleep(50 * time.Millisecond)
		}

		// Calculate mock token counts
		inputTokens := 0
		for _, p := range conversation {
			inputTokens += len(p.Text) / 4
		}
		outputTokens := len(mockText) / 4

		// Send metadata
		stream <- &gen.StreamResponse{
			Type: gen.TYPE_METADATA,
			Metadata: &models.Metadata{
				Model:        g.request.Model.FQN(),
				InputTokens:  inputTokens,
				OutputTokens: outputTokens,
				TotalTokens:  inputTokens + outputTokens,
			},
		}

		// Send EOF
		stream <- &gen.StreamResponse{
			Type:    gen.TYPE_EOF,
			Content: "",
		}

		g.mock.log("[gen] stream completed", "model", g.request.Model.FQN())
	}()

	return stream, nil
}

// Helper functions

func hashString(s string) int64 {
	var hash int64
	for i, c := range s {
		hash = hash*31 + int64(c) + int64(i)
	}
	if hash < 0 {
		hash = -hash
	}
	return hash
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
