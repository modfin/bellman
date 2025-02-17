package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/modfin/bellman/models"
	"github.com/modfin/bellman/models/gen"
	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/tools"
	"io"
	"net/http"
	"sync/atomic"
)

var requestNo int64

type generator struct {
	openai  *OpenAI
	request gen.Request
}

func (g *generator) SetRequest(config gen.Request) {
	g.request = config
}

func (g *generator) Prompt(conversation ...prompt.Prompt) (*gen.Response, error) {
	reqModel := genRequest{
		Model: g.request.Model.Name,
		Stop:  g.request.StopSequences,

		MaxTokens:        g.request.MaxTokens,
		FrequencyPenalty: g.request.FrequencyPenalty,
		PresencePenalty:  g.request.PresencePenalty,
		Temperature:      g.request.Temperature,
		TopP:             g.request.TopP,
	}

	if g.request.Model.Name == "" {
		return nil, fmt.Errorf("model is required")
	}

	toolBelt := map[string]*tools.Tool{}
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
		toolBelt[t.Name] = &t
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

	messages := []genRequestMessage{}

	// Dealing with Prompt Messages
	// Open Ai specific
	if g.request.SystemPrompt != "" {
		messages = append(messages, genRequestMessageText{
			Role:    "system",
			Content: []genRequestMessageContent{{Type: "text", Text: g.request.SystemPrompt}},
		})
	}
	for _, c := range conversation {
		switch c.Role {
		case prompt.ToolResponseRole:
			if c.ToolResponse == nil {
				return nil, fmt.Errorf("ToolResponse is required for role tool response")
			}
			messages = append(messages, genRequestMessageToolResponse{
				Role:       "tool",
				ToolCallID: c.ToolResponse.ToolCallID,
				Content:    c.ToolResponse.Response,
			})
		case prompt.ToolCallRole:
			if c.ToolCall == nil {
				return nil, fmt.Errorf("ToolCall is required for role tool call")
			}
			messages = append(messages, genRequestMessageToolCalls{
				Role: "assistant",
				ToolCalls: []genRequestMessageToolCall{
					{
						ID:   c.ToolCall.ToolCallID,
						Type: "function",
						Function: genRequestMessageToolCallFunction{
							Name:      c.ToolCall.Name,
							Arguments: c.ToolCall.Arguments,
						},
					},
				},
			})
		default: // prompt.UserRole, prompt.AssistantRole
			message := genRequestMessageText{
				Role: string(c.Role),
				Content: []genRequestMessageContent{
					{Type: "text", Text: c.Text},
				},
			}

			if c.Payload != nil {
				message.Content = append(message.Content,
					genRequestMessageContent{
						Type: "image_url",
						ImageUrl: ImageUrl{
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
		return nil, fmt.Errorf("could not marshal open ai request, %w", err)
	}

	u := `https://api.openai.com/v1/chat/completions`

	ctx := g.request.Context
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("could not create openai request, %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+g.openai.apiKey)
	req.Header.Set("Content-Type", "application/json")

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
	)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not post openai request, %w", err)
	}
	defer resp.Body.Close()
	body, err = io.ReadAll(resp.Body)
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
	if len(respModel.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	res := &gen.Response{
		Metadata: models.Metadata{
			Model:        g.request.Model.FQN(),
			InputTokens:  respModel.Usage.PromptTokens,
			OutputTokens: respModel.Usage.CompletionTokens,
			TotalTokens:  respModel.Usage.TotalTokens,
		},
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
					Ref:      toolBelt[t.Function.Name],
				})
			}
		}
	}

	g.openai.log("[gen] response",
		"request", reqc,
		"model", g.request.Model.FQN(),
		"token-input", res.Metadata.InputTokens,
		"token-output", res.Metadata.OutputTokens,
		"token-total", res.Metadata.TotalTokens,
	)

	return res, nil
}
