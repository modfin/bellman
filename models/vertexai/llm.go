package vertexai

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/modfin/bellman"
	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/schema"
	"github.com/modfin/bellman/tools"
	"io"
	"net/http"
)

type generator struct {
	g *Google

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

func (b *generator) clone() *generator {
	var bb generator
	bb = *b
	if b.schema != nil {
		cp := *b.schema
		bb.schema = &cp
	}
	if b.tool != nil {
		cp := *b.tool
		bb.tool = &cp
	}
	if b.tools != nil {
		bb.tools = append([]tools.Tool{}, b.tools...)
	}

	return &bb
}

func (b *generator) Model(model bellman.GenModel) bellman.Generator {
	bb := b.clone()
	bb.model = model
	return bb
}

func (b *generator) System(prompt string) bellman.Generator {
	bb := b.clone()
	bb.systemPrompt = prompt
	return bb
}

func (b *generator) Output(element any) bellman.Generator {
	bb := b.clone()
	bb.schema = schema.New(element)
	return bb
}
func (g *generator) Tools() []tools.Tool {
	return g.tools
}

func (b *generator) SetTools(tool ...tools.Tool) bellman.Generator {
	bb := b.clone()

	bb.tools = append([]tools.Tool{}, tool...)
	return bb
}
func (g *generator) AddTools(tool ...tools.Tool) bellman.Generator {
	return g.SetTools(append(g.tools, tool...)...)
}

func (b *generator) SetToolConfig(tool tools.Tool) bellman.Generator {
	bb := b.clone()
	bb.tool = &tool

	for _, t := range tools.ControlTools {
		if t.Name == tool.Name {
			return bb
		}
	}
	bb.tools = []tools.Tool{tool}
	return bb
}

func (b *generator) StopAt(stop ...string) bellman.Generator {
	bb := b.clone()
	bb.stopSequences = append([]string{}, stop...)

	if len(bb.stopSequences) > 5 {
		bb.stopSequences = bb.stopSequences[:5]
	}

	return bb
}

func (b *generator) Temperature(temperature float64) bellman.Generator {
	bb := b.clone()
	bb.temperature = temperature

	return bb
}

func (b *generator) TopP(topP float64) bellman.Generator {
	bb := b.clone()
	bb.topP = topP

	return bb
}

func (b *generator) MaxTokens(maxTokens int) bellman.Generator {
	bb := b.clone()
	bb.maxTokens = maxTokens

	return bb
}

func (g *generator) Prompt(prompts ...prompt.Prompt) (bellman.Response, error) {

	//https://cloud.google.com/vertex-ai/generative-ai/docs/model-reference/inference

	if g.model.Name == "" {
		return nil, errors.New("model is required")
	}

	model := genRequest{
		Contents:         []genRequestContent{},
		GenerationConfig: genConfig{},
	}

	if g.maxTokens != -1 {
		model.GenerationConfig.MaxOutputTokens = &g.maxTokens
	}
	if g.topP != -1 {
		model.GenerationConfig.TopP = &g.topP
	}
	if g.temperature != -1 {
		model.GenerationConfig.Temperature = &g.temperature
	}
	if len(g.stopSequences) > 0 {
		model.GenerationConfig.StopSequences = &g.stopSequences
	}

	if g.systemPrompt != "" {
		model.SystemInstruction = genRequestContent{
			Role: "system", // does not take role into account, it can be anything?
			Parts: []genRequestContentPart{
				{
					Text: g.systemPrompt,
				},
			},
		}
	}

	// Adding output schema to model
	if g.schema != nil {
		ct := "application/json"
		model.GenerationConfig.ResponseMimeType = &ct
		model.GenerationConfig.ResponseSchema = g.schema
	}

	// Adding tools to model
	if len(g.tools) > 0 {
		model.Tools = []genTool{{FunctionDeclaration: []genToolFunc{}}}
		for _, t := range g.tools {
			model.Tools[0].FunctionDeclaration = append(model.Tools[0].FunctionDeclaration, genToolFunc{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.ArgumentSchema,
			})
		}
	}

	// Dealing with SetToolConfig config
	if g.tool != nil {
		model.ToolConfig = &genToolConfig{
			GoogleFunctionCallingConfig: genFunctionCallingConfig{
				Mode: "ANY",
			},
		}
		// https://cloud.google.com/vertex-ai/generative-ai/docs/model-reference/function-calling#functioncallingconfig
		switch g.tool.Name {
		case tools.NoTool.Name:
			model.ToolConfig.GoogleFunctionCallingConfig.Mode = "NONE"
		case tools.AutoTool.Name:
			model.ToolConfig.GoogleFunctionCallingConfig.Mode = "AUTO"
		case tools.RequiredTool.Name:
			model.ToolConfig.GoogleFunctionCallingConfig.Mode = "ANY"
		default:
			model.ToolConfig.GoogleFunctionCallingConfig.Mode = "ANY"
			model.ToolConfig.GoogleFunctionCallingConfig.AllowedFunctionNames = []string{g.tool.Name}
		}
	}

	for _, p := range prompts {
		var role string
		switch p.Role {
		case prompt.Assistant:
			role = "model"
		default:
			role = "user"
		}
		content := genRequestContent{
			Role: role,
			Parts: []genRequestContentPart{
				{
					Text: p.Text,
				},
			},
		}

		if p.Payload != nil {
			content.Parts[0].InlineDate = &inlineDate{
				MimeType: p.Payload.Mime,
				Data:     p.Payload.Data,
			}
		}

		model.Contents = append(model.Contents, content)
	}

	u := fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s:generateContent",
		g.g.config.Region, g.g.config.Project, g.g.config.Region, g.model.Name)

	body, err := json.Marshal(model)
	if err != nil {
		return nil, fmt.Errorf("could not marshal google request, %w", err)
	}
	resp, err := g.g.client.Post(u, "application/json", bytes.NewReader(body))

	if err != nil {
		return nil, fmt.Errorf("could not post google request, %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		b, err := io.ReadAll(resp.Body)
		return nil, errors.Join(fmt.Errorf("unexpected status code, %d, err: {%s}, for url: {%s} ", resp.StatusCode, string(b), u), err)
	}

	defer resp.Body.Close()
	var respModel geminiResponse
	err = json.NewDecoder(resp.Body).Decode(&respModel)
	if err != nil {
		return nil, fmt.Errorf("could not decode google response, %w", err)
	}

	if len(respModel.Candidates) == 0 {
		fmt.Println(respModel)
		return nil, fmt.Errorf("no candidates in response")
	}
	if len(respModel.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no parts in response")
	}

	return &response{
		llm:   respModel,
		tools: g.tools,
	}, nil

}
