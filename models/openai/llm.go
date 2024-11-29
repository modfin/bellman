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

func (b *generator) Tools(tool ...tools.Tool) bellman.Generator {
	bb := b.clone()

	bb.tools = append([]tools.Tool{}, tool...)
	return bb
}

func (b *generator) Tool(tool tools.Tool) bellman.Generator {
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
	if len(bb.stopSequences) > 4 {
		bb.stopSequences = bb.stopSequences[:4]
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

func (b *generator) Prompt(conversation ...prompt.Prompt) (bellman.Response, error) {

	// Open Ai specific
	if b.systemPrompt != "" {
		conversation = append([]prompt.Prompt{{Role: "system", Text: b.systemPrompt}}, conversation...)
	}

	reqModel := genRequest{
		Stop:        b.stopSequences,
		Temperature: b.temperature,
		TopP:        b.topP,
		MaxTokens:   b.maxTokens,
	}

	// Dealing with Model
	if b.model.Name != "" {
		reqModel.Model = b.model.Name
	}

	if b.model.Name == "" {
		return nil, fmt.Errorf("model is required")
	}

	// Dealing with Tools
	for _, t := range b.tools {
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
	if b.tool != nil {
		switch b.tool.Name {
		case tools.NoTool.Name, tools.AutoTool.Name, tools.RequiredTool.Name:
			reqModel.ToolChoice = b.tool.Name
		default:
			reqModel.ToolChoice = requestTool{
				Type: "function",
				Function: toolFunc{
					Name: b.tool.Name,
				},
			}
		}
	}

	// Dealing with Output Schema
	if b.schema != nil {
		reqModel.ResponseFormat = &responseFormat{
			Type: "json_schema",
			ResponseFormatSchema: responseFormatSchema{
				Name:   "response",
				Strict: false,
				Schema: b.schema,
			},
		}
	}

	// Dealing with Prompt Messages
	messages := []genRequestMessage{}
	for _, c := range conversation {
		messages = append(messages, genRequestMessage{
			Role:    string(c.Role),
			Content: c.Text,
		})
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
	req.Header.Set("Authorization", "Bearer "+b.g.apiKey)
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
		tools: b.tools,
		llm:   respModel,
	}, nil
}
