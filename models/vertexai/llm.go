package vertexai

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/modfin/bellman"
	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/tools"
	"io"
	"net/http"
	"sync/atomic"
)

var requestNo int64

type generator struct {
	google *Google
	config bellman.Config
}

func (g *generator) SetConfig(config bellman.Config) {
	g.config = config
}

func (g *generator) log(msg string, args ...any) {
	if g.config.Log == nil {
		return
	}
	g.config.Log.Debug("[bellman/vertex_ai] "+msg, args...)
}

func (g *generator) Prompt(prompts ...prompt.Prompt) (bellman.Response, error) {

	//https://cloud.google.com/vertex-ai/generative-ai/docs/model-reference/inference

	if g.config.Model.Name == "" {
		return nil, errors.New("model is required")
	}

	model := genRequest{
		Contents:         []genRequestContent{},
		GenerationConfig: genConfig{},
	}

	if g.config.MaxTokens != -1 {
		model.GenerationConfig.MaxOutputTokens = &g.config.MaxTokens
	}
	if g.config.TopP != -1 {
		model.GenerationConfig.TopP = &g.config.TopP
	}
	if g.config.Temperature != -1 {
		model.GenerationConfig.Temperature = &g.config.Temperature
	}
	if len(g.config.StopSequences) > 0 {
		model.GenerationConfig.StopSequences = &g.config.StopSequences
	}

	if g.config.SystemPrompt != "" {
		model.SystemInstruction = genRequestContent{
			Role: "system", // does not take role into account, it can be anything?
			Parts: []genRequestContentPart{
				{
					Text: g.config.SystemPrompt,
				},
			},
		}
	}

	// Adding output schema to model
	if g.config.OutputSchema != nil {
		ct := "application/json"
		model.GenerationConfig.ResponseMimeType = &ct
		model.GenerationConfig.ResponseSchema = g.config.OutputSchema
	}

	// Adding tools to model
	if len(g.config.Tools) > 0 {
		model.Tools = []genTool{{FunctionDeclaration: []genToolFunc{}}}
		for _, t := range g.config.Tools {
			model.Tools[0].FunctionDeclaration = append(model.Tools[0].FunctionDeclaration, genToolFunc{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.ArgumentSchema,
			})
		}
	}

	// Dealing with SetToolConfig config
	if g.config.ToolConfig != nil {
		model.ToolConfig = &genToolConfig{
			GoogleFunctionCallingConfig: genFunctionCallingConfig{
				Mode: "ANY",
			},
		}
		// https://cloud.google.com/vertex-ai/generative-ai/docs/model-reference/function-calling#functioncallingconfig
		switch g.config.ToolConfig.Name {
		case tools.NoTool.Name:
			model.ToolConfig.GoogleFunctionCallingConfig.Mode = "NONE"
		case tools.AutoTool.Name:
			model.ToolConfig.GoogleFunctionCallingConfig.Mode = "AUTO"
		case tools.RequiredTool.Name:
			model.ToolConfig.GoogleFunctionCallingConfig.Mode = "ANY"
		default:
			model.ToolConfig.GoogleFunctionCallingConfig.Mode = "ANY"
			model.ToolConfig.GoogleFunctionCallingConfig.AllowedFunctionNames = []string{g.config.ToolConfig.Name}
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
		g.google.config.Region, g.google.config.Project, g.google.config.Region, g.config.Model.Name)

	body, err := json.Marshal(model)
	if err != nil {
		return nil, fmt.Errorf("could not marshal google request, %w", err)
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
		"url", u,
	)

	resp, err := g.google.client.Post(u, "application/json", bytes.NewReader(body))

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
		tools: g.config.Tools,
	}, nil

}
