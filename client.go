package bellman

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/modfin/bellman/models/embed"
	"github.com/modfin/bellman/models/gen"
	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/tools"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"sync/atomic"
)

const Provider = "Bellman"

type Bellman struct {
	Log *slog.Logger `json:"-"`
	url string
	key Key
}

func (g *Bellman) Provider() string {
	return Provider
}

type Key struct {
	Name  string
	Token string
}

func (l Key) String() string {
	return l.Name + "_" + l.Token
}

func New(url string, key Key) *Bellman {
	return &Bellman{
		url: url,
		key: key,
	}

}

func (g *Bellman) log(msg string, args ...any) {
	if g.Log == nil {
		return
	}
	g.Log.Debug("[bellman/bellman] "+msg, args...)
}

var bellmanRequestNo int64

func (v *Bellman) EmbedModels() ([]embed.Model, error) {
	u, err := url.JoinPath(v.url, "embed", "models")
	if err != nil {
		return nil, fmt.Errorf("could not join url %s; %w", v.url, err)
	}
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, fmt.Errorf("could not create bellman request; %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+v.key.String())
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not post bellman request to %s; %w", u, err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read bellman response; %w", err)
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d; %s", res.StatusCode, string(body))
	}

	var models []embed.Model
	err = json.Unmarshal(body, &models)
	if err != nil {
		v.log("[gen] unmarshal response error", "error", err, "body", string(body))
		return nil, fmt.Errorf("could not unmarshal bellman response; %w", err)
	}
	return models, nil
}

func (v *Bellman) GenModels() ([]gen.Model, error) {
	u, err := url.JoinPath(v.url, "gen", "models")
	if err != nil {
		return nil, fmt.Errorf("could not join url %s; %w", v.url, err)
	}
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, fmt.Errorf("could not create bellman request; %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+v.key.String())
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not post bellman request to %s; %w", u, err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read bellman response; %w", err)
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d; %s", res.StatusCode, string(body))
	}

	var models []gen.Model
	err = json.Unmarshal(body, &models)
	if err != nil {
		v.log("[gen] unmarshal response error", "error", err, "body", string(body))
		return nil, fmt.Errorf("could not unmarshal bellman response; %w", err)
	}
	return models, nil
}

func (v *Bellman) Embed(request *embed.Request) (*embed.Response, error) {
	var reqc = atomic.AddInt64(&bellmanRequestNo, 1)

	u, err := url.JoinPath(v.url, "embed")
	if err != nil {
		return nil, fmt.Errorf("could not join url %s; %w", v.url, err)
	}

	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("could not marshal bellman request; %w", err)
	}

	ctx := request.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("could not create bellman request; %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+v.key.String())
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not post bellman request to %s; %w", u, err)
	}
	defer res.Body.Close()

	body, err = io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read bellman response; %w", err)
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d; %s", res.StatusCode, string(body))
	}

	var response embed.Response
	err = json.Unmarshal(body, &response)
	if err != nil {
		v.log("[gen] unmarshal response error", "error", err, "body", string(body))
		return nil, fmt.Errorf("could not unmarshal bellman response; %w", err)
	}

	v.log("[embed] response", "request", reqc, "model", request.Model.FQN(), "token-total", response.Metadata.TotalTokens)

	return &response, nil
}
func (v *Bellman) EmbedDocument(request *embed.DocumentRequest) (*embed.DocumentResponse, error) {
	var reqc = atomic.AddInt64(&bellmanRequestNo, 1)

	u, err := url.JoinPath(v.url, "embed", "document")
	if err != nil {
		return nil, fmt.Errorf("could not join url %s; %w", v.url, err)
	}

	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("could not marshal bellman request; %w", err)
	}

	ctx := request.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("could not create bellman request; %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+v.key.String())
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not post bellman request to %s; %w", u, err)
	}
	defer res.Body.Close()

	body, err = io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read bellman response; %w", err)
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d; %s", res.StatusCode, string(body))
	}

	var response embed.DocumentResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		v.log("[gen] unmarshal response error", "error", err, "body", string(body))
		return nil, fmt.Errorf("could not unmarshal bellman response; %w", err)
	}

	v.log("[embed] response", "request", reqc, "model", request.Model.FQN(), "token-total", response.Metadata.TotalTokens)

	return &response, nil
}

func (a *Bellman) Generator(options ...gen.Option) *gen.Generator {
	var gen = &gen.Generator{
		Prompter: &generator{
			bellman: a,
		},
		Request: gen.Request{},
	}
	for _, op := range options {
		gen = op(gen)
	}
	return gen
}

func (g *Bellman) SetLogger(logger *slog.Logger) *Bellman {
	g.Log = logger
	return g
}

type generator struct {
	bellman *Bellman
	request gen.Request
}

func (g *generator) SetRequest(request gen.Request) {
	g.request = request
}

func (g *generator) Prompt(conversation ...prompt.Prompt) (*gen.Response, error) {
	var reqc = atomic.AddInt64(&bellmanRequestNo, 1)

	u, err := url.JoinPath(g.bellman.url, "gen")
	if err != nil {
		return nil, fmt.Errorf("could not join url %s; %w", g.bellman.url, err)
	}
	request := gen.FullRequest{
		Request: g.request,
		Prompts: conversation,
	}

	toolBelt := map[string]*tools.Tool{}
	for _, tool := range g.request.Tools {
		toolBelt[tool.Name] = tool
	}

	g.bellman.log("[gen] request",
		"request", reqc,
		"model", g.request.Model.FQN(),
		"tools", len(g.request.Tools) > 0,
		"tool_choice", g.request.ToolChoice != nil,
		"output_schema", g.request.OutputSchema != nil,
		"system_prompt", g.request.SystemPrompt != "",
		"temperature", g.request.Temperature,
		"top_p", g.request.TopP,
		"max_tokens", g.request.MaxTokens,
		"stop_sequences", g.request.StopSequences,
	)

	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("could not marshal bellman request; %w", err)
	}

	ctx := g.request.Context
	if ctx == nil {
		ctx = context.Background()
	}

	req, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("could not create bellman request; %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.bellman.key.String())

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not post bellman request to %s; %w", u, err)
	}
	defer res.Body.Close()

	body, err = io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read bellman response; %w", err)
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d; %s", res.StatusCode, string(body))
	}
	response := gen.Response{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		g.bellman.log("[gen] unmarshal response error", "error", err, "body", string(body))
		return nil, fmt.Errorf("could not unmarshal bellman response; %w", err)
	}

	g.bellman.log("[gen] response",
		"request", reqc,
		"model", g.request.Model.FQN(),
		"token-input", response.Metadata.InputTokens,
		"token-output", response.Metadata.OutputTokens,
		"token-total", response.Metadata.TotalTokens,
	)

	// adding reference to tools
	for i, _ := range response.Tools {
		tool := response.Tools[i]
		tool.Ref = toolBelt[tool.Name]
		response.Tools[i] = tool
	}

	return &response, nil

}

func (g *generator) Stream(conversation ...prompt.Prompt) (<-chan *gen.StreamResponse, error) {
	var reqc = atomic.AddInt64(&bellmanRequestNo, 1)

	// Build streaming request with proper formatting
	request, toolBelt, err := g.buildStreamingRequest(conversation)
	if err != nil {
		return nil, fmt.Errorf("could not build streaming request; %w", err)
	}

	u, err := url.JoinPath(g.bellman.url, "gen", "stream")
	if err != nil {
		return nil, fmt.Errorf("could not get streaming endpoint: %w", err)
	}

	g.bellman.log("[gen] stream request",
		"request", reqc,
		"model", g.request.Model.FQN(),
		"tools", len(g.request.Tools) > 0,
		"tool_choice", g.request.ToolChoice != nil,
		"output_schema", g.request.OutputSchema != nil,
		"system_prompt", g.request.SystemPrompt != "",
		"temperature", g.request.Temperature,
		"top_p", g.request.TopP,
		"max_tokens", g.request.MaxTokens,
		"stop_sequences", g.request.StopSequences,
		"stream", true,
	)

	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("could not marshal bellman request; %w", err)
	}

	ctx := g.request.Context
	if ctx == nil {
		ctx = context.Background()
	}

	req, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("could not create bellman request; %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.bellman.key.String())
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	client := g.createStreamingHTTPClient()
	res, err := client.Do(req)
	if err != nil {
		return nil, g.handleStreamingError(fmt.Errorf("could not post bellman request to %s; %w", u, err), reqc)
	}

	if res.StatusCode != http.StatusOK {
		b, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return nil, g.handleStreamingError(fmt.Errorf("unexpected status code, %d, and failed to read response body: %w", res.StatusCode, readErr), reqc)
		}
		return nil, g.handleStreamingError(fmt.Errorf("unexpected status code, %d, err: {%s}", res.StatusCode, string(b)), reqc)
	}

	reader := bufio.NewReader(res.Body)
	stream := make(chan *gen.StreamResponse, 100)

	go func() {
		defer res.Body.Close()
		defer close(stream)

		defer func() {
			stream <- &gen.StreamResponse{
				Type: gen.TYPE_EOF,
			}
		}()

		// Handle context cancellation
		ctx := g.request.Context
		if ctx == nil {
			ctx = context.Background()
		}

		for {
			// Check for context cancellation
			select {
			case <-ctx.Done():
				g.bellman.log("[gen] stream cancelled by context", "request", reqc, "error", ctx.Err())
				stream <- &gen.StreamResponse{
					Type:    gen.TYPE_ERROR,
					Content: fmt.Sprintf("stream cancelled: %v", ctx.Err()),
				}
				return
			default:
				// Continue processing
			}

			line, _, err := reader.ReadLine()
			if err != nil {
				// If there's an error, check if it's EOF (end of stream)
				if errors.Is(err, http.ErrBodyReadAfterClose) {
					g.bellman.log("[gen] stream closed by server (Read after close)", "request", reqc)
					break
				}
				if errors.Is(err, io.EOF) {
					g.bellman.log("[gen] stream ended (EOF)", "request", reqc)
					break
				}
				g.bellman.log("[gen] error reading from stream", "request", reqc, "error", err)
				stream <- &gen.StreamResponse{
					Type:    gen.TYPE_ERROR,
					Content: fmt.Sprintf("error reading stream: %v", err),
				}
				break // Exit the loop on any other error
			}

			if len(line) == 0 {
				continue
			}
			if !bytes.HasPrefix(line, []byte("data: ")) {
				stream <- &gen.StreamResponse{
					Type:    gen.TYPE_ERROR,
					Content: "expected 'data' header from sse",
				}
				break
			}
			line = line[6:] // removing header

			if bytes.Equal(line, []byte("[DONE]")) {
				g.bellman.log("[gen] stream completed", "request", reqc)
				break // Exit the loop on end of stream
			}

			var streamResp gen.StreamResponse
			err = json.Unmarshal(line, &streamResp)
			if err != nil {
				g.bellman.log("[gen] could not unmarshal stream chunk", "request", reqc, "error", err, "line", string(line))
				stream <- &gen.StreamResponse{
					Type:    gen.TYPE_ERROR,
					Content: fmt.Sprintf("could not unmarshal stream chunk: %v", err),
				}
				break
			}

			// Process the streaming response
			g.processStreamingResponse(&streamResp, toolBelt, reqc)

			// Send the response to the stream
			select {
			case stream <- &streamResp:
				// Successfully sent
			case <-ctx.Done():
				// Context was cancelled while trying to send
				g.bellman.log("[gen] stream cancelled while sending response", "request", reqc, "error", ctx.Err())
				return
			}
		}
	}()

	return stream, nil
}

// buildStreamingRequest creates a properly formatted streaming request
func (g *generator) buildStreamingRequest(conversation []prompt.Prompt) (gen.FullRequest, map[string]*tools.Tool, error) {
	request := gen.FullRequest{
		Request: g.request,
		Prompts: conversation,
	}

	// Ensure streaming is enabled
	request.Stream = true

	// Validate request parameters for streaming
	if err := g.validateStreamingRequest(&request); err != nil {
		return request, nil, err
	}

	// Build tool belt for tool call references
	toolBelt := map[string]*tools.Tool{}
	for _, tool := range g.request.Tools {
		toolBelt[tool.Name] = tool
	}

	return request, toolBelt, nil
}

// validateStreamingRequest validates request parameters for streaming
func (g *generator) validateStreamingRequest(request *gen.FullRequest) error {
	if request.Model.Name == "" {
		return fmt.Errorf("model is required for streaming request")
	}

	// Validate that we have prompts
	if len(request.Prompts) == 0 {
		return fmt.Errorf("at least one prompt is required for streaming request")
	}

	// Validate tool configuration if tools are present
	if len(request.Tools) > 0 && request.ToolChoice != nil {
		// Check if the specified tool exists
		toolExists := false
		for _, tool := range request.Tools {
			if tool.Name == request.ToolChoice.Name {
				toolExists = true
				break
			}
		}
		if !toolExists {
			return fmt.Errorf("specified tool '%s' not found in available tools", request.ToolChoice.Name)
		}
	}

	return nil
}

// createStreamingHTTPClient creates an HTTP client optimized for streaming
func (g *generator) createStreamingHTTPClient() *http.Client {
	// Use a longer timeout for streaming requests
	transport := &http.Transport{
		DisableCompression: true,  // Disable compression for streaming
		DisableKeepAlives:  false, // Keep connections alive for streaming
	}

	return &http.Client{
		Transport: transport,
		// No timeout for streaming - let context handle cancellation
	}
}

// isRetryableError checks if an error is retryable for streaming requests
func (g *generator) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for network-related errors that might be retryable
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	// Check for temporary network errors
	var netErr *net.OpError
	if errors.As(err, &netErr) {
		return netErr.Temporary()
	}

	return false
}

// handleStreamingError handles streaming-specific errors
func (g *generator) handleStreamingError(err error, reqc int64) error {
	if g.isRetryableError(err) {
		g.bellman.log("[gen] retryable streaming error", "request", reqc, "error", err)
		return fmt.Errorf("retryable streaming error: %w", err)
	}

	g.bellman.log("[gen] streaming error", "request", reqc, "error", err)
	return fmt.Errorf("streaming error: %w", err)
}

// processStreamingResponse processes a streaming response and adds necessary references
func (g *generator) processStreamingResponse(streamResp *gen.StreamResponse, toolBelt map[string]*tools.Tool, reqc int64) {
	// Add tool references for tool calls
	if streamResp.ToolCall != nil && streamResp.ToolCall.Ref == nil {
		if tool, exists := toolBelt[streamResp.ToolCall.Name]; exists {
			streamResp.ToolCall.Ref = tool
		}
	}

	// Log metrics if metadata is present
	if streamResp.Type == gen.TYPE_METADATA && streamResp.Metadata != nil {
		g.bellman.log("[gen] stream metrics",
			"request", reqc,
			"model", g.request.Model.FQN(),
			"token-input", streamResp.Metadata.InputTokens,
			"token-output", streamResp.Metadata.OutputTokens,
			"token-total", streamResp.Metadata.TotalTokens,
		)
	}
}
