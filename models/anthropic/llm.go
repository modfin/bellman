package anthropic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/modfin/bellman"
	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/schema"
	"github.com/modfin/bellman/tools"
	"io"
	"net/http"
	"strings"
)

type generator struct {
	a *Anthropic
	// Cloneable...
	model        bellman.GenModel
	systemPrompt string

	stopSequences []string
	topP          float64
	temperature   float64
	maxTokens     int

	schema *schema.JSON
	tools  []tools.Tool
	tool   *tools.Tool
}

func (g *generator) clone() *generator {
	var bb generator
	bb = *g
	if g.schema != nil {
		cp := *g.schema
		bb.schema = &cp
	}
	if g.tool != nil {
		cp := *g.tool
		bb.tool = &cp
	}
	if g.tools != nil {
		bb.tools = append([]tools.Tool{}, g.tools...)
	}

	return &bb
}
func (g *generator) Tools() []tools.Tool {
	return g.tools
}

func (g *generator) SetTools(tool ...tools.Tool) bellman.Generator {
	bb := g.clone()

	bb.tools = append([]tools.Tool{}, tool...)
	return bb
}

func (g *generator) AddTools(tool ...tools.Tool) bellman.Generator {
	return g.SetTools(append(g.tools, tool...)...)
}

func (g *generator) SetToolConfig(tool tools.Tool) bellman.Generator {
	bb := g.clone()
	bb.tool = &tool

	for _, t := range tools.ControlTools {
		if t.Name == tool.Name {
			return bb
		}
	}
	bb.tools = []tools.Tool{tool}
	return bb
}

func (g *generator) StopAt(stop ...string) bellman.Generator {
	bb := g.clone()
	bb.stopSequences = append([]string{}, stop...)
	if len(bb.stopSequences) > 4 {
		bb.stopSequences = bb.stopSequences[:4]
	}

	return bb
}

func (g *generator) Temperature(temperature float64) bellman.Generator {
	bb := g.clone()
	bb.temperature = temperature

	return bb
}

func (g *generator) TopP(topP float64) bellman.Generator {
	bb := g.clone()
	bb.topP = topP

	return bb
}

func (g *generator) MaxTokens(maxTokens int) bellman.Generator {
	bb := g.clone()
	bb.maxTokens = maxTokens

	return bb
}

func (g *generator) Model(model bellman.GenModel) bellman.Generator {
	bb := g.clone()
	bb.model = model
	return bb
}

func (g *generator) System(prompt string) bellman.Generator {
	bb := g.clone()
	bb.systemPrompt = prompt
	return bb
}

func (g *generator) Output(element any) bellman.Generator {
	bb := g.clone()
	bb.schema = schema.New(element)
	return bb
}

func (g *generator) Prompt(conversation ...prompt.Prompt) (bellman.Response, error) {

	var pdfBeta bool

	model := request{
		Model:       g.model.Name,
		Temperature: g.temperature,
		MaxTokens:   g.maxTokens,
	}

	if g.temperature != -1 {
		model.Temperature = g.temperature
	}
	if g.topP != -1 {
		model.TopP = &g.topP
	}
	if g.systemPrompt != "" {
		model.System = g.systemPrompt
	}

	if len(g.stopSequences) > 0 {
		model.StopSequences = g.stopSequences
	}

	if g.schema != nil {
		model.Tools = []reqTool{
			{
				Name:        respone_output_callback_name,
				Description: "function that is called with the result of the llm query",
				InputSchema: g.schema,
			},
		}
		model.Tool = &reqToolChoice{
			Type: "tool",
			Name: respone_output_callback_name,
		}
	}

	if len(g.tools) > 0 {

		model.Tools = nil // If output is specified, tools override it.
		model.Tool = nil

		for _, t := range g.tools {
			model.Tools = append(model.Tools, reqTool{
				Name:        t.Name,
				Description: t.Description,
				InputSchema: t.ArgumentSchema,
			})
		}
	}

	if g.tool != nil {
		_name := ""
		_type := ""

		switch g.tool.Name {
		case tools.NoTool.Name:
		case tools.AutoTool.Name:
			_type = "auto"
		case tools.RequiredTool.Name:
			_type = "any"
		default:
			_type = "tool"
			_name = g.tool.Name
		}
		model.Tool = &reqToolChoice{
			Type: _type, // // "auto, any, tool"
			Name: _name,
		}

		if g.tool.Name == tools.NoTool.Name { // None is not supporded by Anthropic, so lets just remove the toolks.
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
	//if g.schema != nil {
	//	model.Output = g.schema
	//}

	reqdata, err := json.Marshal(model)
	if err != nil {
		return nil, fmt.Errorf("could not marshal request, %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(reqdata))
	if err != nil {
		return nil, fmt.Errorf("could not create request, %w", err)
	}

	req.Header.Set("x-api-key", g.a.apiKey)
	req.Header.Set("anthropic-version", AnthropicVersion)
	req.Header.Set("content-type", "application/json")
	if pdfBeta {
		req.Header.Add("anthropic-beta", "pdfs-2024-09-25")
	}

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
		tools: g.tools,
	}, nil
}
