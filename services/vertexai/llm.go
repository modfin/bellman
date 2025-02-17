package vertexai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/modfin/bellman/models"
	"github.com/modfin/bellman/models/gen"
	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/schema"
	"github.com/modfin/bellman/tools"
	"io"
	"net/http"
	"sync/atomic"
)

var requestNo int64

type generator struct {
	google  *Google
	request gen.Request
}

func (g *generator) SetRequest(config gen.Request) {
	g.request = config
}

func (g *generator) Prompt(prompts ...prompt.Prompt) (*gen.Response, error) {

	//https://cloud.google.com/vertex-ai/generative-ai/docs/model-reference/inference

	if g.request.Model.Name == "" {
		return nil, errors.New("model is required")
	}

	model := genRequest{
		Contents: []genRequestContent{},
		GenerationConfig: &genConfig{
			MaxOutputTokens:  g.request.MaxTokens,
			TopP:             g.request.TopP,
			TopK:             g.request.TopK,
			Temperature:      g.request.Temperature,
			StopSequences:    g.request.StopSequences,
			FrequencyPenalty: g.request.FrequencyPenalty,
			PresencePenalty:  g.request.PresencePenalty,
		},
	}

	if g.request.SystemPrompt != "" {
		model.SystemInstruction = &genRequestContent{
			Role: "system", // does not take role into account, it can be anything?
			Parts: []genRequestContentPart{
				{
					Text: g.request.SystemPrompt,
				},
			},
		}
	}

	// Adding output schema to model
	if g.request.OutputSchema != nil {
		var outputSchemaDefs map[string]*schema.JSON
		if g.request.OutputSchema.Defs != nil {
			outputSchemaDefs = g.request.OutputSchema.Defs
		}
		ct := "application/json"
		model.GenerationConfig.ResponseMimeType = &ct
		model.GenerationConfig.ResponseSchema = fromBellmanSchema(g.request.OutputSchema, outputSchemaDefs)
	}

	// Adding tools to model

	toolBelt := map[string]*tools.Tool{}
	if len(g.request.Tools) > 0 {
		model.Tools = []genTool{{FunctionDeclaration: []genToolFunc{}}}
		for _, t := range g.request.Tools {
			var toolSchemaDefs map[string]*schema.JSON
			if t.ArgumentSchema != nil {
				toolSchemaDefs = t.ArgumentSchema.Defs
			}
			model.Tools[0].FunctionDeclaration = append(model.Tools[0].FunctionDeclaration, genToolFunc{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  fromBellmanSchema(t.ArgumentSchema, toolSchemaDefs),
			})
			toolBelt[t.Name] = &t
		}
	}

	// Dealing with SetToolConfig request
	if g.request.ToolConfig != nil {
		model.ToolConfig = &genToolConfig{
			GoogleFunctionCallingConfig: genFunctionCallingConfig{
				Mode: "ANY",
			},
		}
		// https://cloud.google.com/vertex-ai/generative-ai/docs/model-reference/function-calling#functioncallingconfig
		switch g.request.ToolConfig.Name {
		case tools.NoTool.Name:
			model.ToolConfig.GoogleFunctionCallingConfig.Mode = "NONE"
		case tools.AutoTool.Name:
			model.ToolConfig.GoogleFunctionCallingConfig.Mode = "AUTO"
		case tools.RequiredTool.Name:
			model.ToolConfig.GoogleFunctionCallingConfig.Mode = "ANY"
		default:
			model.ToolConfig.GoogleFunctionCallingConfig.Mode = "ANY"
			model.ToolConfig.GoogleFunctionCallingConfig.AllowedFunctionNames = []string{g.request.ToolConfig.Name}
		}
	}

	for _, p := range prompts {
		content := genRequestContent{
			Parts: []genRequestContentPart{},
		}
		switch p.Role {
		case prompt.ToolResponseRole:
			if p.ToolResponse == nil {
				return nil, fmt.Errorf("ToolResponse is required for role tool response")
			}
			content.Role = "tool"
			content.Parts = append(content.Parts, genRequestContentPart{
				FunctionResponse: &functionResponse{
					Name: p.ToolResponse.Name,
					Response: struct {
						Name    string `json:"name,omitempty"`
						Content any    `json:"content,omitempty"`
					}{Name: p.ToolResponse.Name, Content: p.ToolResponse.Response},
				},
			})
		case prompt.ToolCallRole:
			if p.ToolCall == nil {
				return nil, fmt.Errorf("ToolCall is required for role tool call")
			}
			content.Role = "model"
			var jsonArguments map[string]any
			err := json.Unmarshal(p.ToolCall.Arguments, &jsonArguments)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal tool call arguments: %w", err)
			}
			content.Parts = append(content.Parts, genRequestContentPart{
				FunctionCall: &functionCall{Name: p.ToolCall.Name, Args: jsonArguments},
			})
		default: // prompt.UserRole, prompt.AssistantRole
			content.Role = "user"
			if p.Role == prompt.AssistantRole {
				content.Role = "model"
			}
			part := genRequestContentPart{Text: p.Text}
			if p.Payload != nil {
				if len(p.Payload.Data) > 0 {
					part.InlineData = &inlineData{
						MimeType: p.Payload.Mime,
						Data:     p.Payload.Data,
					}
				}
				if len(p.Payload.Uri) > 0 {
					part.InlineData = nil
					part.FileData = &fileData{
						MimeType: p.Payload.Mime,
						FileUri:  p.Payload.Uri,
					}
				}
			}
			content.Parts = append(content.Parts, part)
		}
		model.Contents = append(model.Contents, content)
	}

	region := g.google.config.Region
	project := g.google.config.Project
	if len(g.request.Model.Config) > 0 {
		cfg := g.request.Model.Config

		r, ok := cfg["region"].(string)
		if ok {
			region = r
		}
		p, ok := cfg["project"].(string)
		if ok {
			project = p
		}
	}

	u := fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s:generateContent",
		region, project, region, g.request.Model.Name)

	body, err := json.Marshal(model)
	if err != nil {
		return nil, fmt.Errorf("could not marshal google request, %w", err)
	}

	reqc := atomic.AddInt64(&requestNo, 1)
	g.google.log("[gen] request",
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
		"url", u,
	)

	ctx := g.request.Context
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("could not create google request, %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := g.google.client.Do(req)

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

	res := &gen.Response{
		Metadata: models.Metadata{
			Model:        g.request.Model.FQN(),
			InputTokens:  respModel.UsageMetadata.PromptTokenCount,
			OutputTokens: respModel.UsageMetadata.CandidatesTokenCount,
			TotalTokens:  respModel.UsageMetadata.TotalTokenCount,
		},
	}
	for _, c := range respModel.Candidates {
		for _, p := range c.Content.Parts {
			if len(p.Text) > 0 {
				res.Texts = append(res.Texts, p.Text)
			}

			if len(p.Text) == 0 && len(p.FunctionCall.Name) > 0 { // Tool calls
				f := p.FunctionCall
				arg, err := json.Marshal(f.Args)
				if err != nil {
					return nil, fmt.Errorf("could not marshal google request, %w", err)
				}
				res.Tools = append(res.Tools, tools.Call{
					Name:     f.Name,
					Argument: arg,
					Ref:      toolBelt[f.Name],
				})

			}

		}
	}

	g.google.log("[gen] response",
		"request", reqc,
		"model", g.request.Model.FQN(),
		"token-input", res.Metadata.InputTokens,
		"token-output", res.Metadata.OutputTokens,
		"token-total", res.Metadata.TotalTokens,
	)

	return res, nil

}
