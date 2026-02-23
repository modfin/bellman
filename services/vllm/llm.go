package vllm

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
	vllm    *VLLM
	request gen.Request
}

func (g *generator) SetRequest(config gen.Request) {
	g.request = config
}

func (g *generator) Stream(conversation ...prompt.Prompt) (<-chan *gen.StreamResponse, error) {
	g.request.Stream = true
	req, reqModel, err := g.prompt(conversation...)
	if err != nil {
		return nil, err
	}

	reqc := atomic.AddInt64(&requestNo, 1)
	g.vllm.log("[gen] request",
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
		return nil, fmt.Errorf("could not post vllm request, %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		b, err := io.ReadAll(resp.Body)
		return nil, errors.Join(fmt.Errorf("unexpected status code, %d, err: {%s}", resp.StatusCode, string(b)), err)
	}

	reader := bufio.NewReader(resp.Body)

	stream := make(chan *gen.StreamResponse)

	go func() {
		defer resp.Body.Close()
		defer close(stream)

		defer func() {
			stream <- &gen.StreamResponse{
				Type: gen.TYPE_EOF,
			}
		}()

		var role string
		var toolName string
		var toolCallID string
		for {
			line, _, err := reader.ReadLine()
			if err != nil {
				// If there's an error, check if it's EOF (end of stream)
				if errors.Is(err, http.ErrBodyReadAfterClose) {
					log.Println("SSE stream closed by server (Read after close).")
					break
				}
				log.Printf("Error reading from stream: %v", err)
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
				break // Exit the loop on any other error
			}

			//fmt.Printf("LINE: %s\n", string(line))

			var ss vllmStreamResponse
			err = json.Unmarshal(line, &ss)
			if err != nil {
				log.Printf("could not unmarshal chunk, %v", err)
				break
			}

			if ss.Usage != nil {
				m := &models.Metadata{
					Model:          ss.Model,
					InputTokens:    ss.Usage.PromptTokens,
					OutputTokens:   ss.Usage.CompletionTokens,
					ThinkingTokens: ss.Usage.CompletionTokensDetails.ReasoningTokens,
					TotalTokens:    ss.Usage.TotalTokens,
				}
				if ss.ServiceTier != nil {
					m.Other = map[string]any{"service_tier": *ss.ServiceTier}
				}
				stream <- &gen.StreamResponse{
					Type:     gen.TYPE_METADATA,
					Metadata: m,
				}
				continue
			}

			if ss.ServiceTier != nil {
				g.vllm.log("[gen] stream resp, service tier", "msg", string(line), "service_tier", *ss.ServiceTier)
			}

			if len(ss.Choices) == 0 { // Something wrong
				// Should never really get here...
				g.vllm.log("[gen] stream request, no choices", "msg", string(line))
				stream <- &gen.StreamResponse{
					Type:    gen.TYPE_ERROR,
					Content: "there where no choises in response",
				}
				break
			}

			for _, choice := range ss.Choices {
				// eg, "finish_reason":"stop", discard and wait for usage
				if choice.FinishReason != nil { // ignore it...
					continue
				}
				if len(choice.Delta.Role) > 0 {
					role = choice.Delta.Role // Probably usually check, assistant
				}
				if choice.Delta.Content != nil {
					stream <- &gen.StreamResponse{
						Type:    gen.TYPE_DELTA,
						Role:    prompt.Role(role),
						Index:   choice.Index,
						Content: *choice.Delta.Content,
					}
				}
				if len(choice.Delta.ToolCalls) > 0 {
					for _, toolCall := range choice.Delta.ToolCalls {
						if len(toolCall.Function.Name) > 0 && len(toolCall.ID) > 0 {
							toolName = toolCall.Function.Name
							toolCallID = toolCall.ID
						}
						if len(toolName) == 0 || len(toolCallID) == 0 {
							stream <- &gen.StreamResponse{
								Type:    gen.TYPE_ERROR,
								Content: "tool call without name or id",
							}
							continue
						}

						stream <- &gen.StreamResponse{
							Type:  gen.TYPE_DELTA,
							Role:  prompt.ToolCallRole,
							Index: choice.Index,
							ToolCall: &tools.Call{
								ID:       toolCallID,
								Name:     toolName,
								Argument: []byte(toolCall.Function.Arguments),
								Ref:      reqModel.toolBelt[toolName],
							},
						}
					}
				}
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
	g.vllm.log("[gen] request",
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
		return nil, fmt.Errorf("could not post vllm request, %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read vllm response, %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code, %d: err %s", resp.StatusCode, string(body))
	}

	var respModel vllmResponse
	err = json.Unmarshal(body, &respModel)
	if err != nil {
		return nil, fmt.Errorf("could not decode vllm response, %w", err)
	}
	if len(respModel.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	res := &gen.Response{
		Metadata: models.Metadata{
			Model:          g.request.Model.FQN(),
			InputTokens:    respModel.Usage.PromptTokens,
			OutputTokens:   respModel.Usage.CompletionTokens,
			ThinkingTokens: respModel.Usage.CompletionTokensDetails.ReasoningTokens,
			TotalTokens:    respModel.Usage.TotalTokens,
		},
	}
	if respModel.ServiceTier != nil {
		g.vllm.log("[gen] prompt resp, service tier", "service_tier", *respModel.ServiceTier)
		if res.Metadata.Other == nil {
			res.Metadata.Other = map[string]any{}
		}
		res.Metadata.Other["service_tier"] = *respModel.ServiceTier
	}
	for _, c := range respModel.Choices {
		message := c.Message
		if len(message.ToolCalls) == 0 { // Not Tools
			res.Texts = append(res.Texts, c.Message.Content)
		}

		if len(message.ToolCalls) > 0 { // ToolResponseRole calls
			for _, t := range message.ToolCalls {
				res.Tools = append(res.Tools, tools.Call{
					ID:       t.ID,
					Name:     t.Function.Name,
					Argument: []byte(t.Function.Arguments),
					Ref:      reqModel.toolBelt[t.Function.Name],
				})
			}
		}
	}

	g.vllm.log("[gen] response",
		"request", reqc,
		"model", g.request.Model.FQN(),
		"token-input", res.Metadata.InputTokens,
		"token-output", res.Metadata.OutputTokens,
		"token-thinking", res.Metadata.ThinkingTokens,
		"token-total", res.Metadata.TotalTokens,
	)

	return res, nil
}

func (g *generator) prompt(conversation ...prompt.Prompt) (*http.Request, genRequest, error) {
	reqModel := genRequest{
		Stream: g.request.Stream,
		Model:  g.request.Model.Name,
		Stop:   g.request.StopSequences,

		MaxTokens:        g.request.MaxTokens,
		FrequencyPenalty: g.request.FrequencyPenalty,
		PresencePenalty:  g.request.PresencePenalty,
		Temperature:      g.request.Temperature,
		TopP:             g.request.TopP,
	}

	uri := g.vllm.modelToUri[g.request.Model.Name]
	if uri == "" && g.vllm.modelToUri["*"] == "" {
		return nil, reqModel, fmt.Errorf("model %s not found", g.request.Model.Name)
	}
	if uri == "" {
		uri = g.vllm.modelToUri["*"]
	}

	if reqModel.Stream {
		reqModel.StreamOptions = &StreamOptions{IncludeUsage: true}
	}

	if g.request.Model.Name == "" {
		return nil, reqModel, fmt.Errorf("model is required")
	}

	if len(g.request.Model.Config) > 0 {
		if v, ok := g.request.Model.Config["service_tier"]; ok {
			switch fmt.Sprintf("%v", v) {
			case "auto":
				st := ServiceTierAuto
				reqModel.ServiceTier = &st
			case "default":
				st := ServiceTierDefault
				reqModel.ServiceTier = &st
			case "flex":
				st := ServiceTierFlex
				reqModel.ServiceTier = &st
			case "priority":
				st := ServiceTierPriority
				reqModel.ServiceTier = &st
			default:
				return nil, reqModel, fmt.Errorf("unknown service tier: %s", v)
			}
		}
	}

	reqModel.toolBelt = map[string]*tools.Tool{}
	// Dealing with Tools
	for _, t := range g.request.Tools {
		reqModel.Tools = append(reqModel.Tools, requestTool{
			Type: "function",
			Function: toolFunc{
				Name:        t.Name,
				Parameters:  fromBellmanSchema(t.ArgumentSchema),
				Description: t.Description,
				Strict:      g.request.StrictOutput,
			},
		})
		reqModel.toolBelt[t.Name] = &t
	}
	// Selecting specific tool
	if g.request.ToolConfig != nil {
		switch g.request.ToolConfig.Name {
		case tools.NoTool.Name, tools.AutoTool.Name, tools.RequiredTool.Name:
			reqModel.ToolChoice = g.request.ToolConfig.Name
		default:
			reqModel.ToolChoice = requestTool{
				Type: "function",
				Function: toolFunc{
					Name: g.request.ToolConfig.Name,
				},
			}
		}
	}

	// Dealing with Output Schema
	if g.request.OutputSchema != nil {
		reqModel.ResponseFormat = &responseFormat{
			Type: "json_schema",
			ResponseFormatSchema: responseFormatSchema{
				Name:   "response",
				Strict: g.request.StrictOutput,
				Schema: fromBellmanSchema(g.request.OutputSchema),
			},
		}
	}

	if g.request.ThinkingBudget != nil && *g.request.ThinkingBudget > 0 {
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
		reqModel.ReasoningEffort = &reffort
	}

	messages := []genRequestMessage{}

	// Dealing with Prompt Messages
	// Open Ai specific
	if g.request.SystemPrompt != "" {
		messages = append(messages, genRequestMessageText{
			Role:    "system",
			Content: []genRequestMessageContent{{Type: "text", Text: &g.request.SystemPrompt}},
		})
	}
	for _, c := range conversation {
		switch c.Role {
		case prompt.ToolResponseRole:
			if c.ToolResponse == nil {
				return nil, reqModel, fmt.Errorf("ToolResponse is required for role tool response")
			}
			messages = append(messages, genRequestMessageToolResponse{
				Role:       "tool",
				ToolCallID: c.ToolResponse.ToolCallID,
				Content:    c.ToolResponse.Response,
			})
		case prompt.ToolCallRole:
			if c.ToolCall == nil {
				return nil, reqModel, fmt.Errorf("ToolCall is required for role tool call")
			}
			var jsonArguments map[string]any
			if err := json.Unmarshal(c.ToolCall.Arguments, &jsonArguments); err != nil {
				return nil, reqModel, fmt.Errorf("ToolCall.Arguments is not valid JSON object: %w", err)
			}
			messages = append(messages, genRequestMessageToolCalls{
				Role: "assistant",
				ToolCalls: []genRequestMessageToolCall{
					{
						ID:   c.ToolCall.ToolCallID,
						Type: "function",
						Function: genRequestMessageToolCallFunction{
							Name:      c.ToolCall.Name,
							Arguments: jsonArguments,
						},
					},
				},
			})
		default: // prompt.UserRole, prompt.AssistantRole
			message := genRequestMessageText{
				Role: string(c.Role),
				Content: []genRequestMessageContent{
					{Type: "text", Text: &c.Text},
				},
			}

			if c.Payload != nil {
				message.Content = append(message.Content,
					genRequestMessageContent{
						Type: "image_url",
						ImageUrl: &ImageUrl{
							Url:  c.Payload.Uri,
							data: c.Payload.Data,
						},
					},
				)
			}
			messages = append(messages, message)
		}
	}
	reqModel.Messages = messages

	body, err := json.Marshal(reqModel)
	if err != nil {
		return nil, reqModel, fmt.Errorf("could not marshal open ai request, %w", err)
	}

	u, err := url.JoinPath(uri, "/v1/chat/completions")
	if err != nil {
		return nil, genRequest{}, fmt.Errorf("could not join url, %w", err)
	}

	ctx := g.request.Context
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(body))
	if err != nil {
		return nil, reqModel, fmt.Errorf("could not create vllm request, %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	return req, reqModel, err
}
