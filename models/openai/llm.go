package openai

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
	"os"
)

type generator struct {
	g *OpenAI
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

func (g *generator) Tools() []tools.Tool {
	return g.tools
}

func (g *generator) AddTools(tool ...tools.Tool) bellman.Generator {
	return g.SetTools(append(g.tools, tool...)...)
}
func (g *generator) SetTools(tool ...tools.Tool) bellman.Generator {
	bb := g.clone()

	bb.tools = append([]tools.Tool{}, tool...)
	return bb
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

	// Open Ai specific
	if g.systemPrompt != "" {
		conversation = append([]prompt.Prompt{{Role: "system", Text: g.systemPrompt}}, conversation...)
	}

	reqModel := genRequest{
		Stop:        g.stopSequences,
		Temperature: g.temperature,
		TopP:        g.topP,
		MaxTokens:   g.maxTokens,
	}

	// Dealing with Model
	if g.model.Name != "" {
		reqModel.Model = g.model.Name
	}

	if g.model.Name == "" {
		return nil, fmt.Errorf("model is required")
	}

	// Dealing with Tools
	for _, t := range g.tools {
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
	if g.tool != nil {
		switch g.tool.Name {
		case tools.NoTool.Name, tools.AutoTool.Name, tools.RequiredTool.Name:
			reqModel.ToolChoice = g.tool.Name
		default:
			reqModel.ToolChoice = requestTool{
				Type: "function",
				Function: toolFunc{
					Name: g.tool.Name,
				},
			}
		}
	}

	// Dealing with Output Schema
	if g.schema != nil {
		reqModel.ResponseFormat = &responseFormat{
			Type: "json_schema",
			ResponseFormatSchema: responseFormatSchema{
				Name:   "response",
				Strict: false,
				Schema: g.schema,
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
	req.Header.Set("Authorization", "Bearer "+g.g.apiKey)
	req.Header.Set("Content-Type", "application/json")

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
		tools: g.tools,
		llm:   respModel,
	}, nil
}
