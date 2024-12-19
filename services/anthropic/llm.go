package anthropic

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
	"strings"
	"sync/atomic"
)

var requestNo int64

type generator struct {
	anthropic *Anthropic
	request   gen.Request
}

func (g *generator) SetRequest(config gen.Request) {
	g.request = config
}

func (g *generator) Prompt(conversation ...prompt.Prompt) (*gen.Response, error) {

	var pdfBeta bool

	model := request{
		Model:     g.request.Model.Name,
		MaxTokens: 1024,

		// Optionals..
		Temperature:   g.request.Temperature,
		TopP:          g.request.TopP,
		TopK:          g.request.TopK,
		System:        g.request.SystemPrompt,
		StopSequences: g.request.StopSequences,
	}

	if g.request.MaxTokens != nil && *g.request.MaxTokens > 0 {
		model.MaxTokens = *g.request.MaxTokens
	}

	if g.request.OutputSchema != nil {
		model.Tools = []reqTool{
			{
				Name:        respone_output_callback_name,
				Description: "function that is called with the result of the llm query",
				InputSchema: g.request.OutputSchema,
			},
		}
		model.Tool = &reqToolChoice{
			Type: "tool",
			Name: respone_output_callback_name,
		}
	}

	toolBelt := map[string]*tools.Tool{}
	if len(g.request.Tools) > 0 {
		model.Tools = nil // If output is specified, tools override it.
		model.Tool = nil

		for _, t := range g.request.Tools {
			model.Tools = append(model.Tools, reqTool{
				Name:        t.Name,
				Description: t.Description,
				InputSchema: t.ArgumentSchema,
			})
			toolBelt[t.Name] = &t
		}
	}

	if g.request.ToolConfig != nil {
		_name := ""
		_type := ""

		switch g.request.ToolConfig.Name {
		case tools.NoTool.Name:
		case tools.AutoTool.Name:
			_type = "auto"
		case tools.RequiredTool.Name:
			_type = "any"
		default:
			_type = "tool"
			_name = g.request.ToolConfig.Name
		}
		model.Tool = &reqToolChoice{
			Type: _type, // // "auto, any, tool"
			Name: _name,
		}

		if g.request.ToolConfig.Name == tools.NoTool.Name { // None is not supporded by Anthropic, so lets just remove the toolks.
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

	ctx := g.request.Context
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(reqdata))
	if err != nil {
		return nil, fmt.Errorf("could not create request, %w", err)
	}

	req.Header.Set("x-api-key", g.anthropic.apiKey)
	req.Header.Set("anthropic-version", Version)
	req.Header.Set("content-type", "application/json")
	if pdfBeta {
		req.Header.Add("anthropic-beta", "pdfs-2024-09-25")
	}

	reqc := atomic.AddInt64(&requestNo, 1)
	g.anthropic.log("[gen] request",
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
		"anthropic-version", Version,
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

		if c.Type == "tool_use" {
			arg, err := json.Marshal(c.Input)
			if err != nil {
				return nil, fmt.Errorf("could not marshal tool arguments, %w", err)
			}
			res.Tools = append(res.Tools, tools.Call{
				Name:     c.Name,
				Argument: string(arg),
				Ref:      toolBelt[c.Name],
			})
		}
	}

	// This is really an out put schema callback. So lets just transform it to Text
	if len(res.Tools) == 1 && res.Tools[0].Name == respone_output_callback_name {
		res.Texts = []string{res.Tools[0].Argument}
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
