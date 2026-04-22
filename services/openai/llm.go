package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sync/atomic"

	"github.com/modfin/bellman/models"
	"github.com/modfin/bellman/models/gen"
	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/tools"
)

var requestNo int64

type generator struct {
	openai  *OpenAI
	request gen.Request
}

func (g *generator) SetRequest(config gen.Request) {
	g.request = config
}

type streamingFunctionCall struct {
	CallID string
	Name   string
}

func (g *generator) Stream(conversation ...prompt.Prompt) (<-chan *gen.StreamResponse, error) {
	g.request.Stream = true
	req, reqModel, err := g.prompt(conversation...)
	if err != nil {
		return nil, err
	}

	reqc := atomic.AddInt64(&requestNo, 1)
	g.openai.log("[gen] request",
		"request", reqc,
		"model", g.request.Model.FQN(),
		"tools", len(g.request.Tools) > 0,
		"tool_choice", g.request.ToolConfig != nil,
		"output_schema", g.request.OutputSchema != nil,
		"system_prompt", g.request.SystemPrompt != "",
		"temperature", g.request.Temperature,
		"top_p", g.request.TopP,
		"max_tokens", g.request.MaxTokens,
		"stop_sequences", g.request.StopSequences,
		"thinking_budget", g.request.ThinkingBudget != nil,
		"thinking_parts", g.request.ThinkingParts != nil,
	)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not post openai request, %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		b, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, errors.Join(fmt.Errorf("unexpected status code, %d, err: {%s}", resp.StatusCode, string(b)), err)
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)

	stream := make(chan *gen.StreamResponse)

	go func() {
		defer resp.Body.Close()
		defer close(stream)

		defer func() {
			stream <- &gen.StreamResponse{
				Type: gen.TYPE_EOF,
			}
		}()

		functionCalls := map[int]streamingFunctionCall{}

		for scanner.Scan() {
			line := scanner.Bytes()

			if len(line) == 0 {
				continue
			}
			if bytes.HasPrefix(line, []byte("event: ")) {
				continue
			}
			if bytes.HasPrefix(line, []byte(":")) {
				continue
			}
			if !bytes.HasPrefix(line, []byte("data: ")) {
				stream <- &gen.StreamResponse{
					Type:    gen.TYPE_ERROR,
					Content: "expected 'data' header from sse",
				}
				break
			}
			line = line[6:]

			if bytes.Equal(line, []byte("[DONE]")) {
				break
			}

			var ev streamEvent
			if err := json.Unmarshal(line, &ev); err != nil {
				log.Printf("could not unmarshal chunk, %v", err)
				break
			}

			switch ev.Type {
			case "response.output_item.added":
				if ev.Item != nil && ev.Item.Type == "function_call" {
					functionCalls[ev.OutputIndex] = streamingFunctionCall{
						CallID: ev.Item.CallID,
						Name:   ev.Item.Name,
					}
				}

			case "response.output_text.delta":
				if ev.Delta == "" {
					continue
				}
				stream <- &gen.StreamResponse{
					Type:    gen.TYPE_DELTA,
					Role:    prompt.AssistantRole,
					Index:   ev.OutputIndex,
					Content: ev.Delta,
				}

			case "response.function_call_arguments.delta":
				fc, ok := functionCalls[ev.OutputIndex]
				if !ok || fc.CallID == "" || fc.Name == "" {
					stream <- &gen.StreamResponse{
						Type:    gen.TYPE_ERROR,
						Content: "tool call without name or id",
					}
					continue
				}
				stream <- &gen.StreamResponse{
					Type:  gen.TYPE_DELTA,
					Role:  prompt.ToolCallRole,
					Index: ev.OutputIndex,
					ToolCall: &tools.Call{
						ID:       fc.CallID,
						Name:     fc.Name,
						Argument: []byte(ev.Delta),
						Ref:      reqModel.toolBelt[fc.Name],
					},
				}

			case "response.reasoning_summary_text.delta":
				if g.request.ThinkingParts == nil || !*g.request.ThinkingParts {
					continue
				}
				if ev.Delta == "" {
					continue
				}
				stream <- &gen.StreamResponse{
					Type:    gen.TYPE_THINKING_DELTA,
					Role:    prompt.AssistantRole,
					Index:   ev.OutputIndex,
					Content: ev.Delta,
				}

			case "response.completed":
				if ev.Response != nil {
					if ev.Response.ServiceTier != nil {
						g.openai.log("[gen] stream resp, service tier", "service_tier", *ev.Response.ServiceTier)
					}
					stream <- &gen.StreamResponse{
						Type:     gen.TYPE_METADATA,
						Metadata: responseToMetadata(ev.Response),
					}
				}
				return

			case "response.failed":
				msg := "response failed"
				if ev.Response != nil && ev.Response.Error != nil && ev.Response.Error.Message != "" {
					msg = ev.Response.Error.Message
				}
				stream <- &gen.StreamResponse{Type: gen.TYPE_ERROR, Content: msg}
				return

			case "response.incomplete":
				reason := "response incomplete"
				if ev.Response != nil && ev.Response.IncompleteDetails != nil && ev.Response.IncompleteDetails.Reason != "" {
					reason = "response incomplete: " + ev.Response.IncompleteDetails.Reason
				}
				stream <- &gen.StreamResponse{Type: gen.TYPE_ERROR, Content: reason}
				return

			case "error":
				msg := ev.Message
				if msg == "" {
					msg = "stream error"
				}
				stream <- &gen.StreamResponse{Type: gen.TYPE_ERROR, Content: msg}
				return

			default:
				// Ignore: response.created, response.in_progress, response.queued,
				// response.content_part.*, response.output_text.done, response.output_item.done,
				// response.function_call_arguments.done, response.refusal.*, reasoning_summary_part.*,
				// MCP/web_search/file_search/code_interpreter/image_generation/audio events.
			}
		}
		if err := scanner.Err(); err != nil {
			log.Printf("Error reading from stream: %v", err)
			stream <- &gen.StreamResponse{
				Type:    gen.TYPE_ERROR,
				Content: fmt.Sprintf("sse read error: %v", err),
			}
		}

	}()

	return stream, nil

}
func (g *generator) Prompt(conversation ...prompt.Prompt) (*gen.Response, error) {

	req, reqModel, err := g.prompt(conversation...)
	if err != nil {
		return nil, err
	}

	reqc := atomic.AddInt64(&requestNo, 1)
	g.openai.log("[gen] request",
		"request", reqc,
		"model", g.request.Model.FQN(),
		"tools", len(g.request.Tools) > 0,
		"tool_choice", g.request.ToolConfig != nil,
		"output_schema", g.request.OutputSchema != nil,
		"system_prompt", g.request.SystemPrompt != "",
		"temperature", g.request.Temperature,
		"top_p", g.request.TopP,
		"max_tokens", g.request.MaxTokens,
		"stop_sequences", g.request.StopSequences,
		"thinking_budget", g.request.ThinkingBudget != nil,
		"thinking_parts", g.request.ThinkingParts != nil,
	)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not post openai request, %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read openai response, %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code, %d: err %s", resp.StatusCode, string(body))
	}

	var respModel openaiResponse
	err = json.Unmarshal(body, &respModel)
	if err != nil {
		return nil, fmt.Errorf("could not decode openai response, %w", err)
	}

	if respModel.Status != "" && respModel.Status != "completed" {
		if respModel.Error != nil && respModel.Error.Message != "" {
			return nil, fmt.Errorf("openai response status %s: %s", respModel.Status, respModel.Error.Message)
		}
		if respModel.IncompleteDetails != nil && respModel.IncompleteDetails.Reason != "" {
			return nil, fmt.Errorf("openai response status %s: %s", respModel.Status, respModel.IncompleteDetails.Reason)
		}
		return nil, fmt.Errorf("openai response status %s", respModel.Status)
	}

	res := &gen.Response{
		Metadata: *responseToMetadata(&respModel),
	}
	res.Metadata.Model = g.request.Model.FQN()

	if respModel.ServiceTier != nil {
		g.openai.log("[gen] prompt resp, service tier", "service_tier", *respModel.ServiceTier)
	}

	for _, item := range respModel.Output {
		switch item.Type {
		case "message":
			for _, c := range item.Content {
				if c.Type == "output_text" && c.Text != "" {
					res.Texts = append(res.Texts, c.Text)
				}
			}
		case "function_call":
			res.Tools = append(res.Tools, tools.Call{
				ID:       item.CallID,
				Name:     item.Name,
				Argument: []byte(item.Arguments),
				Ref:      reqModel.toolBelt[item.Name],
			})
		case "reasoning":
			if g.request.ThinkingParts == nil || !*g.request.ThinkingParts {
				continue
			}
			for _, s := range item.Summary {
				if s.Text != "" {
					res.Thinking = append(res.Thinking, s.Text)
				}
			}
		}
	}

	g.openai.log("[gen] response",
		"request", reqc,
		"model", g.request.Model.FQN(),
		"token-input", res.Metadata.InputTokens,
		"token-output", res.Metadata.OutputTokens,
		"token-thinking", res.Metadata.ThinkingTokens,
		"token-total", res.Metadata.TotalTokens,
	)

	return res, nil
}

func responseToMetadata(r *openaiResponse) *models.Metadata {
	thinking := r.Usage.OutputTokensDetails.ReasoningTokens
	output := r.Usage.OutputTokens - thinking
	if output < 0 {
		output = 0
	}
	m := &models.Metadata{
		Model:          r.Model,
		InputTokens:    r.Usage.InputTokens,
		OutputTokens:   output,
		ThinkingTokens: thinking,
		TotalTokens:    r.Usage.TotalTokens,
	}
	if r.ServiceTier != nil {
		m.Other = map[string]any{"service_tier": *r.ServiceTier}
	}
	return m
}

func (g *generator) prompt(conversation ...prompt.Prompt) (*http.Request, genRequest, error) {
	reqModel := genRequest{
		Stream:          g.request.Stream,
		Model:           g.request.Model.Name,
		MaxOutputTokens: g.request.MaxTokens,
		Temperature:     g.request.Temperature,
		TopP:            g.request.TopP,
		Store:           new(false),
	}

	if g.request.Model.Name == "" {
		return nil, reqModel, fmt.Errorf("model is required")
	}

	if len(g.request.StopSequences) > 0 {
		g.openai.log("[gen] dropping stop_sequences (not supported by /v1/responses)", "stop", g.request.StopSequences)
	}
	if g.request.FrequencyPenalty != nil {
		g.openai.log("[gen] dropping frequency_penalty (not supported by /v1/responses)")
	}
	if g.request.PresencePenalty != nil {
		g.openai.log("[gen] dropping presence_penalty (not supported by /v1/responses)")
	}

	if len(g.request.Model.Config) > 0 {
		if v, ok := g.request.Model.Config["service_tier"]; ok {
			switch fmt.Sprintf("%v", v) {
			case "auto":
				reqModel.ServiceTier = new(ServiceTierAuto)
			case "default":
				reqModel.ServiceTier = new(ServiceTierDefault)
			case "flex":
				reqModel.ServiceTier = new(ServiceTierFlex)
			case "priority":
				reqModel.ServiceTier = new(ServiceTierPriority)
			default:
				return nil, reqModel, fmt.Errorf("unknown service tier: %s", v)
			}
		}
	}

	reqModel.toolBelt = map[string]*tools.Tool{}
	for _, t := range g.request.Tools {
		reqModel.Tools = append(reqModel.Tools, responsesTool{
			Type:        "function",
			Name:        t.Name,
			Description: t.Description,
			Parameters:  fromBellmanSchema(t.ArgumentSchema),
			Strict:      g.request.StrictOutput,
		})
		reqModel.toolBelt[t.Name] = &t
	}
	if g.request.ToolConfig != nil {
		switch g.request.ToolConfig.Name {
		case tools.NoTool.Name, tools.AutoTool.Name, tools.RequiredTool.Name:
			reqModel.ToolChoice = g.request.ToolConfig.Name
		default:
			reqModel.ToolChoice = map[string]any{
				"type": "function",
				"name": g.request.ToolConfig.Name,
			}
		}
	}

	if g.request.OutputSchema != nil {
		reqModel.Text = &textConfig{
			Format: &responseTextFormat{
				Type:   "json_schema",
				Name:   "response",
				Strict: g.request.StrictOutput,
				Schema: fromBellmanSchema(g.request.OutputSchema),
			},
		}
	}

	if g.request.ThinkingBudget != nil {
		var reffort ReasoningEffort
		switch true {
		case *g.request.ThinkingBudget == 0:
			reffort = ReasoningEffortNone
		case *g.request.ThinkingBudget < 2_000:
			reffort = ReasoningEffortLow
		case *g.request.ThinkingBudget < 10_000:
			reffort = ReasoningEffortMedium
		default:
			reffort = ReasoningEffortHigh
		}
		reqModel.Reasoning = &reasoningConfig{Effort: &reffort}
	}
	if g.request.ThinkingParts != nil && *g.request.ThinkingParts {
		if reqModel.Reasoning == nil {
			reqModel.Reasoning = &reasoningConfig{}
		}
		reqModel.Reasoning.Summary = new("auto")
	}

	if g.request.SystemPrompt != "" {
		reqModel.Instructions = new(g.request.SystemPrompt)
	}

	var input []inputItem
	for _, c := range conversation {
		switch c.Role {
		case prompt.ToolResponseRole:
			if c.ToolResponse == nil {
				return nil, reqModel, fmt.Errorf("ToolResponse is required for role tool response")
			}
			input = append(input, functionCallOutputItem{
				Type:   "function_call_output",
				CallID: c.ToolResponse.ToolCallID,
				Output: c.ToolResponse.Response,
			})
		case prompt.ToolCallRole:
			if c.ToolCall == nil {
				return nil, reqModel, fmt.Errorf("ToolCall is required for role tool call")
			}
			var jsonArguments map[string]any
			if err := json.Unmarshal(c.ToolCall.Arguments, &jsonArguments); err != nil {
				return nil, reqModel, fmt.Errorf("ToolCall.Arguments is not valid JSON object: %w", err)
			}
			input = append(input, functionCallItem{
				Type:      "function_call",
				CallID:    c.ToolCall.ToolCallID,
				Name:      c.ToolCall.Name,
				Arguments: string(c.ToolCall.Arguments),
			})
		default: // prompt.UserRole, prompt.AssistantRole
			contentType := "input_text"
			if c.Role == prompt.AssistantRole {
				contentType = "output_text"
			}
			item := messageItem{
				Role: string(c.Role),
				Content: []messageContent{
					{Type: contentType, Text: new(c.Text)},
				},
			}
			if c.Payload != nil {
				item.Content = append(item.Content, messageContent{
					Type:     "input_image",
					ImageURL: new(imagePayloadURL(c.Payload)),
				})
			}
			input = append(input, item)
		}
	}
	reqModel.Input = input

	body, err := json.Marshal(reqModel)
	if err != nil {
		return nil, reqModel, fmt.Errorf("could not marshal open ai request, %w", err)
	}

	u, err := url.JoinPath(g.openai.getBaseURL(), "/v1/responses")
	if err != nil {
		return nil, reqModel, fmt.Errorf("could not construct responses URL, %w", err)
	}

	ctx := g.request.Context
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(body))
	if err != nil {
		return nil, reqModel, fmt.Errorf("could not create openai request, %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+g.openai.apiKey)
	req.Header.Set("Content-Type", "application/json")

	return req, reqModel, err
}

func imagePayloadURL(p *prompt.Payload) string {
	if p.Uri != "" {
		return p.Uri
	}
	mime := p.Mime
	if mime == "" {
		mime = "image/jpeg"
	}
	return "data:" + mime + ";base64," + p.Data
}
