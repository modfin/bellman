package anthropic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/modfin/bellman"
	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/tools"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
)

var requestNo int64

type generator struct {
	anthropic *Anthropic
	config    bellman.Config
}

func (g *generator) SetConfig(config bellman.Config) {
	g.config = config
}

func (g *generator) log(msg string, args ...any) {
	if g.config.Log == nil {
		return
	}
	g.config.Log.Debug("[bellman/anthropic] "+msg, args...)
}

func (g *generator) Prompt(conversation ...prompt.Prompt) (bellman.Response, error) {

	var pdfBeta bool

	model := request{
		Model:       g.config.Model.Name,
		Temperature: g.config.Temperature,
		MaxTokens:   g.config.MaxTokens,
	}

	if g.config.Temperature != -1 {
		model.Temperature = g.config.Temperature
	}
	if g.config.TopP != -1 {
		model.TopP = &g.config.TopP
	}
	if g.config.SystemPrompt != "" {
		model.System = g.config.SystemPrompt
	}

	if len(g.config.StopSequences) > 0 {
		model.StopSequences = g.config.StopSequences
	}

	if g.config.OutputSchema != nil {
		model.Tools = []reqTool{
			{
				Name:        respone_output_callback_name,
				Description: "function that is called with the result of the llm query",
				InputSchema: g.config.OutputSchema,
			},
		}
		model.Tool = &reqToolChoice{
			Type: "tool",
			Name: respone_output_callback_name,
		}
	}

	if len(g.config.Tools) > 0 {

		model.Tools = nil // If output is specified, tools override it.
		model.Tool = nil

		for _, t := range g.config.Tools {
			model.Tools = append(model.Tools, reqTool{
				Name:        t.Name,
				Description: t.Description,
				InputSchema: t.ArgumentSchema,
			})
		}
	}

	if g.config.ToolConfig != nil {
		_name := ""
		_type := ""

		switch g.config.ToolConfig.Name {
		case tools.NoTool.Name:
		case tools.AutoTool.Name:
			_type = "auto"
		case tools.RequiredTool.Name:
			_type = "any"
		default:
			_type = "tool"
			_name = g.config.ToolConfig.Name
		}
		model.Tool = &reqToolChoice{
			Type: _type, // // "auto, any, tool"
			Name: _name,
		}

		if g.config.ToolConfig.Name == tools.NoTool.Name { // None is not supporded by Anthropic, so lets just remove the toolks.
			model.Tool = nil
			model.Tools = nil
		}
	}

	for _, t := range conversation {

		message := reqMessages{
			Role: string(t.Role),
			Content: []reqContent{{
				Type: "text",
				Text: t.Text,
			}},
		}

		if t.Payload != nil {
			message.Content[0].Text = ""

			if t.Payload.Mime == "application/pdf" {
				pdfBeta = true
				message.Content[0].Type = "document"
				message.Content[0].Source = &reqContentSource{
					Type:      "base64",
					MediaType: t.Payload.Mime,
					Data:      t.Payload.Data,
				}
			}

			if strings.HasPrefix(t.Payload.Mime, "image/") {
				message.Content[0].Type = "image"
				message.Content[0].Source = &reqContentSource{
					Type:      "base64",
					MediaType: t.Payload.Mime,
					Data:      t.Payload.Data,
				}
			}
		}

		model.Messages = append(model.Messages, message)
	}

	reqdata, err := json.Marshal(model)
	if err != nil {
		return nil, fmt.Errorf("could not marshal request, %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(reqdata))
	if err != nil {
		return nil, fmt.Errorf("could not create request, %w", err)
	}

	req.Header.Set("x-api-key", g.anthropic.apiKey)
	req.Header.Set("anthropic-version", AnthropicVersion)
	req.Header.Set("content-type", "application/json")
	if pdfBeta {
		req.Header.Add("anthropic-beta", "pdfs-2024-09-25")
	}

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
		"anthropic-version", AnthropicVersion,
	)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not post request, %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code, %d, %s", resp.StatusCode, (string(b)))
	}

	var respModel anthropicResponse
	err = json.NewDecoder(resp.Body).Decode(&respModel)
	if err != nil {
		return nil, fmt.Errorf("could not decode response, %w", err)
	}

	if len(respModel.Content) == 0 {
		return nil, fmt.Errorf("no content in response")
	}
	return &response{
		llm:   respModel,
		tools: g.config.Tools,
	}, nil
}
