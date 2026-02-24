package anthropic

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
	"strings"
	"sync/atomic"

	"github.com/modfin/bellman/models"
	"github.com/modfin/bellman/models/gen"
	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/tools"
)

var requestNo int64

type generator struct {
	anthropic *Anthropic
	request   gen.Request
}

func (g *generator) SetRequest(config gen.Request) {
	g.request = config
}
func (g *generator) Stream(conversation ...prompt.Prompt) (<-chan *gen.StreamResponse, error) {
	g.request.Stream = true
	req, reqModel, err := g.prompt(conversation...)
	if err != nil {
		return nil, fmt.Errorf("could not create request: %w", err)
	}

	reqc := atomic.AddInt64(&requestNo, 1)
	g.anthropic.log("[gen] request",
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
		"thinking_budget", g.request.ThinkingBudget != nil,
		"thinking_parts", g.request.ThinkingParts != nil,
		"anthropic-version", Version,
	)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not post openai request, %w", err)
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
		var toolID string
		var toolName string
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
			if bytes.HasPrefix(line, []byte("event: ")) {
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

			var ss anthropicStreamResponse
			err = json.Unmarshal(line, &ss)
			if err != nil {
				log.Printf("could not unmarshal chunk, %v", err)
				break
			}

			if ss.Type == "message_stop" {
				// This is the end of a message, we can reset the role and tool ID
				role = ""
				toolID = ""
				toolName = ""
				return
			}

			if ss.Usage != nil {
				stream <- &gen.StreamResponse{
					Type: gen.TYPE_METADATA,
					Metadata: &models.Metadata{
						Model:        g.request.Model.Name,
						InputTokens:  ss.Usage.InputTokens,
						OutputTokens: ss.Usage.OutputTokens,
					},
				}

			}
			if ss.Message != nil && (ss.Message.Usage.InputTokens != 0 || ss.Message.Usage.OutputTokens != 0) {
				stream <- &gen.StreamResponse{
					Type: gen.TYPE_METADATA,
					Metadata: &models.Metadata{
						Model:        ss.Message.Model,
						InputTokens:  ss.Message.Usage.InputTokens,
						OutputTokens: ss.Message.Usage.OutputTokens,
					},
				}
			}

			if ss.Message != nil {
				// This is a message start
				if ss.Message.Role == "assistant" || ss.Message.Role == "user" {
					role = ss.Message.Role
				} else {
					role = "assistant" // Default to assistant if role is not set
				}
				if len(ss.Message.Content) > 0 {
					for _, content := range ss.Message.Content {
						if len(content.Text) == 0 {
							continue
						}
						stream <- &gen.StreamResponse{
							Type:    gen.TYPE_DELTA,
							Role:    prompt.Role(role),
							Index:   ss.Index,
							Content: content.Text,
						}
					}
				}
			}
			if ss.ContentBlock != nil {
				if ss.ContentBlock.Type == "tool_use" && ss.ContentBlock.ID != nil && ss.ContentBlock.Name != nil {
					toolID = *ss.ContentBlock.ID
					toolName = *ss.ContentBlock.Name
				}
			}
			if ss.Delta != nil {
				if len(toolID) > 0 && len(toolName) > 0 && ss.Delta.PartialJSON != nil {
					if toolName == respone_output_callback_name {
						// If the tool is the output callback, we just send the partial JSON as text
						stream <- &gen.StreamResponse{
							Type:    gen.TYPE_DELTA,
							Role:    prompt.AssistantRole,
							Index:   ss.Index,
							Content: *ss.Delta.PartialJSON,
						}
					} else {
						stream <- &gen.StreamResponse{
							Type:  gen.TYPE_DELTA,
							Role:  prompt.ToolCallRole,
							Index: ss.Index,
							ToolCall: &tools.Call{
								ID:       toolID,
								Name:     toolName,
								Argument: []byte(*ss.Delta.PartialJSON),
								Ref:      reqModel.toolBelt[toolName],
							},
						}
					}

				}
				if ss.Delta.Text != nil && len(*ss.Delta.Text) > 0 {
					stream <- &gen.StreamResponse{
						Type:    gen.TYPE_DELTA,
						Role:    prompt.Role(role),
						Index:   ss.Index,
						Content: *ss.Delta.Text,
					}
				}
				if ss.Delta.Thinking != nil && len(*ss.Delta.Thinking) > 0 {
					stream <- &gen.StreamResponse{
						Type:    gen.TYPE_THINKING_DELTA,
						Role:    prompt.AssistantRole,
						Index:   ss.Index,
						Content: *ss.Delta.Thinking,
					}
				}
			}
			if ss.Type == "content_block_stop" {
				toolID = ""   // Reset tool ID on content block stop
				toolName = "" // Reset tool name on content block stop
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
	g.anthropic.log("[gen] request",
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
		"thinking_budget", g.request.ThinkingBudget != nil,
		"thinking_parts", g.request.ThinkingParts != nil,
		"anthropic-version", Version,
	)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not post request, %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, err := io.ReadAll(resp.Body)
		return nil, errors.Join(fmt.Errorf("unexpected status code, %d, err: {%s}", resp.StatusCode, string(b)), err)
	}

	var respModel anthropicResponse
	err = json.NewDecoder(resp.Body).Decode(&respModel)
	if err != nil {
		return nil, fmt.Errorf("could not decode response, %w", err)
	}

	if len(respModel.Content) == 0 {
		return nil, fmt.Errorf("no content in response")
	}

	res := &gen.Response{
		Metadata: models.Metadata{
			Model:        g.request.Model.FQN(),
			InputTokens:  respModel.Usage.InputTokens,
			OutputTokens: respModel.Usage.OutputTokens,
			TotalTokens:  respModel.Usage.InputTokens + respModel.Usage.OutputTokens,
		},
	}
	for _, c := range respModel.Content {
		if c.Type == "text" { // Not Tools
			res.Texts = append(res.Texts, c.Text)
		}
		if c.Type == "thinking" {
			res.Thinking = append(res.Thinking, c.Thinking)
		}

		if c.Type == "tool_use" {
			arg, err := json.Marshal(c.Input)
			if err != nil {
				return nil, fmt.Errorf("could not marshal tool arguments, %w", err)
			}
			res.Tools = append(res.Tools, tools.Call{
				ID:       c.ID,
				Name:     c.Name,
				Argument: arg,
				Ref:      reqModel.toolBelt[c.Name],
			})
		}
	}

	// This is really an output schema callback. So lets just transform it to Text
	if len(res.Tools) == 1 && res.Tools[0].Name == respone_output_callback_name {
		res.Texts = []string{string(res.Tools[0].Argument)}
		res.Tools = nil
	}

	g.anthropic.log("[gen] response",
		"request", reqc,
		"model", g.request.Model.FQN(),
		"token-input", res.Metadata.InputTokens,
		"token-output", res.Metadata.OutputTokens,
		"token-total", res.Metadata.TotalTokens,
	)

	return res, nil
}
func (g *generator) prompt(conversation ...prompt.Prompt) (*http.Request, request, error) {
	var pdfBeta bool

	model := request{
		Stream:    g.request.Stream,
		Model:     g.request.Model.Name,
		MaxTokens: 1024,

		// Optionals..
		Temperature:   g.request.Temperature,
		TopP:          g.request.TopP,
		TopK:          g.request.TopK,
		System:        g.request.SystemPrompt,
		StopSequences: g.request.StopSequences,
		toolBelt:      make(map[string]*tools.Tool),
	}

	if g.request.MaxTokens != nil && *g.request.MaxTokens > 0 {
		model.MaxTokens = *g.request.MaxTokens
	}

	if g.request.OutputSchema != nil {
		model.Tools = []reqTool{
			{
				Name:        respone_output_callback_name,
				Description: "function that is called with the result of the llm query",
				InputSchema: fromBellmanSchema(g.request.OutputSchema),
			},
		}
		model.Tool = &reqToolChoice{
			Type: "any",
		}
	}

	if len(g.request.Tools) > 0 {
		for _, t := range g.request.Tools {
			model.Tools = append(model.Tools, reqTool{
				Name:        t.Name,
				Description: t.Description,
				InputSchema: fromBellmanSchema(t.ArgumentSchema),
			})
			model.toolBelt[t.Name] = t
		}
	}

	if g.request.ToolChoice != nil {
		_name := ""
		_type := ""

		switch g.request.ToolChoice.Mode {
		case tools.ToolModeNone:
			// None is not supported by Anthropic, so remove tools
			model.Tool = nil
			model.Tools = nil
		case tools.ToolModeAuto:
			_type = "auto"
		case tools.ToolModeRequired:
			_type = "any"
		case tools.ToolModeSpecific:
			_type = "tool"
			_name = g.request.ToolChoice.Name
		}
		if model.Tool != nil && g.request.ToolChoice.Mode != tools.ToolModeNone {
			model.Tool = &reqToolChoice{
				Type: _type, // "auto, any, tool"
				Name: _name,
			}
		}
	}

	if g.request.ThinkingBudget != nil {
		model.Thinking = &reqExtendedThinking{
			BudgetTokens: *g.request.ThinkingBudget,
			Type:         ExtendedThinkingTypeEnabled,
		}
	}
	if g.request.ThinkingBudget != nil && *g.request.ThinkingBudget == 0 {
		model.Thinking = &reqExtendedThinking{
			Type: ExtendedThinkingTypeDisabled,
		}
	}

	for _, t := range conversation {
		var message reqMessages
		switch t.Role {
		case prompt.ToolResponseRole:
			if t.ToolResponse == nil {
				return nil, model, fmt.Errorf("ToolResponse is required for role tool response")
			}
			message = reqMessages{
				Role: "user",
				Content: []reqContent{{
					Type:      "tool_result",
					ToolUseID: t.ToolResponse.ToolCallID,
					Content:   t.ToolResponse.Response,
				}},
			}
		case prompt.ToolCallRole:
			if t.ToolCall == nil {
				return nil, model, fmt.Errorf("ToolCall is required for role tool call")
			}
			var jsonArguments map[string]any
			err := json.Unmarshal(t.ToolCall.Arguments, &jsonArguments)
			if err != nil {
				return nil, model, fmt.Errorf("ToolCall.Arguments is not map[string]any: %v", err)
			}
			message = reqMessages{
				Role: "assistant",
				Content: []reqContent{{
					Type:  "tool_use",
					ID:    t.ToolCall.ToolCallID,
					Name:  t.ToolCall.Name,
					Input: jsonArguments,
				}},
			}
		default: // prompt.UserRole, prompt.AssistantRole
			message = reqMessages{
				Role:    string(t.Role),
				Content: []reqContent{},
			}
			if t.Text != "" {
				message.Content = append(message.Content, reqContent{
					Type: "text",
					Text: t.Text,
				})
			}
			if t.Payload != nil {
				if t.Payload.Mime == "application/pdf" {
					message.Content = append(message.Content, reqContent{
						Type: "document",
						Source: &reqContentSource{
							Type:      "base64",
							MediaType: t.Payload.Mime,
							Data:      t.Payload.Data,
						},
					})
					pdfBeta = true
				}

				if strings.HasPrefix(t.Payload.Mime, "image/") {
					message.Content = append(message.Content, reqContent{
						Type: "image",
						Source: &reqContentSource{
							Type:      "base64",
							MediaType: t.Payload.Mime,
							Data:      t.Payload.Data,
						},
					})
				}
			}
		}

		model.Messages = append(model.Messages, message)
	}

	reqdata, err := json.Marshal(model)
	if err != nil {
		return nil, model, fmt.Errorf("could not marshal request, %w", err)
	}

	ctx := g.request.Context
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(reqdata))
	if err != nil {
		return nil, model, fmt.Errorf("could not create request, %w", err)
	}

	req.Header.Set("x-api-key", g.anthropic.apiKey)
	req.Header.Set("anthropic-version", Version)
	req.Header.Set("content-type", "application/json")
	if pdfBeta {
		req.Header.Add("anthropic-beta", "pdfs-2024-09-25")
	}
	return req, model, nil
}
