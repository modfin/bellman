package openai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/modfin/bellman"
	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/tools"
	"io"
	"net/http"
	"os"
	"sync/atomic"
)

var requestNo int64

type generator struct {
	openai *OpenAI
	config bellman.Config
}

func (g *generator) SetConfig(config bellman.Config) {
	g.config = config
}

func (g *generator) log(msg string, args ...any) {
	if g.config.Log == nil {
		return
	}
	g.config.Log.Debug("[bellman/open_ai] "+msg, args...)
}

func (g *generator) Prompt(conversation ...prompt.Prompt) (bellman.Response, error) {

	// Open Ai specific
	if g.config.SystemPrompt != "" {
		conversation = append([]prompt.Prompt{{Role: "system", Text: g.config.SystemPrompt}}, conversation...)
	}

	reqModel := genRequest{
		Stop:        g.config.StopSequences,
		Temperature: g.config.Temperature,
		TopP:        g.config.TopP,
		MaxTokens:   g.config.MaxTokens,
	}

	// Dealing with Model
	if g.config.Model.Name != "" {
		reqModel.Model = g.config.Model.Name
	}

	if g.config.Model.Name == "" {
		return nil, fmt.Errorf("Model is required")
	}

	// Dealing with Tools
	for _, t := range g.config.Tools {
		reqModel.Tools = append(reqModel.Tools, requestTool{
			Type: "function",
			Function: toolFunc{
				Name:        t.Name,
				Parameters:  t.ArgumentSchema,
				Description: t.Description,
				Strict:      false,
			},
		})
	}
	// Selecting specific tool
	if g.config.ToolConfig != nil {
		switch g.config.ToolConfig.Name {
		case tools.NoTool.Name, tools.AutoTool.Name, tools.RequiredTool.Name:
			reqModel.ToolChoice = g.config.ToolConfig.Name
		default:
			reqModel.ToolChoice = requestTool{
				Type: "function",
				Function: toolFunc{
					Name: g.config.ToolConfig.Name,
				},
			}
		}
	}

	// Dealing with SetOutputSchema Schema
	if g.config.OutputSchema != nil {
		reqModel.ResponseFormat = &responseFormat{
			Type: "json_schema",
			ResponseFormatSchema: responseFormatSchema{
				Name:   "response",
				Strict: false,
				Schema: g.config.OutputSchema,
			},
		}
	}

	// Dealing with Prompt Messages
	messages := []genRequestMessage{}
	for _, c := range conversation {
		message := genRequestMessage{
			Role: string(c.Role),
			Content: []genMessageContent{
				{Type: "text", Text: c.Text},
			},
		}
		if c.Payload != nil {
			message.Content[0].Type = "image_url"
			message.Content[0].ImageUrl = &ImageUrl{reader: c.Payload.Data}
		}

		messages = append(messages, message)
	}
	reqModel.Messages = messages

	body, err := json.Marshal(reqModel)
	if err != nil {
		return nil, fmt.Errorf("could not marshal open ai request, %w", err)
	}

	u := `https://api.openai.com/v1/chat/completions`

	req, err := http.NewRequest("POST", u, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("could not create openai request, %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+g.openai.apiKey)
	req.Header.Set("Content-Type", "application/json")

	g.log("sending request",
		"request", atomic.AddInt64(&requestNo, 1),
		"model", g.config.Model.Name,
		"tools", len(g.config.Tools) > 0,
		"tool_choice", g.config.ToolConfig != nil,
		"output_schema", g.config.OutputSchema != nil,
		"system_prompt", g.config.SystemPrompt != "",
		"temperature", g.config.Temperature,
		"top_p", g.config.TopP,
		"max_tokens", g.config.MaxTokens,
		"stop_sequences", g.config.StopSequences,
	)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not post openai request, %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(os.Stdout, resp.Body)
		return nil, fmt.Errorf("unexpected status code, %d", resp.StatusCode)
	}

	var respModel openaiResponse
	err = json.NewDecoder(resp.Body).Decode(&respModel)
	if err != nil {
		return nil, fmt.Errorf("could not decode openai response, %w", err)
	}
	if len(respModel.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}
	return &respone{
		tools: g.config.Tools,
		llm:   respModel,
	}, nil
}
