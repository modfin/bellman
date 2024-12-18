package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/modfin/bellman/models"
	"github.com/modfin/bellman/models/gen"
	"github.com/modfin/bellman/prompt"
	"io"
	"net/http"
	"net/url"
	"sync/atomic"
)

var requestNo int64

type generator struct {
	ollama  *Ollama
	request gen.Request
}

func (g *generator) SetRequest(config gen.Request) {
	g.request = config
}

func (g *generator) Prompt(conversation ...prompt.Prompt) (*gen.Response, error) {

	// Open Ai specific
	if g.request.SystemPrompt != "" {
		conversation = append([]prompt.Prompt{{Role: "system", Text: g.request.SystemPrompt}}, conversation...)
	}

	reqModel := genRequest{
		Model:    g.request.Model.Name,
		Messages: nil,
		Option: genRequestOption{
			Temperature: g.request.Temperature,
			TopP:        g.request.TopP,
			TopK:        g.request.TopK,

			MaxTokens: g.request.MaxTokens,

			FrequencyPenalty: g.request.FrequencyPenalty,
			PresencePenalty:  g.request.PresencePenalty,

			StopSequences: g.request.StopSequences,
		},
		Stream: false,
	}

	if g.request.Model.Name == "" {
		return nil, fmt.Errorf("model is required")
	}

	//toolBelt := map[string]*tools.Tool{}
	//// Dealing with Tools
	//for _, t := range g.request.Tools {
	//	reqModel.Tools = append(reqModel.Tools, requestTool{
	//		Type: "function",
	//		Function: toolFunc{
	//			Name:        t.Name,
	//			Parameters:  t.ArgumentSchema,
	//			Description: t.Description,
	//			Strict:      false,
	//		},
	//	})
	//	toolBelt[t.Name] = &t
	//}
	//// Selecting specific tool
	//if g.request.ToolConfig != nil {
	//	switch g.request.ToolConfig.Name {
	//	case tools.NoTool.Name, tools.AutoTool.Name, tools.RequiredTool.Name:
	//		reqModel.ToolChoice = g.request.ToolConfig.Name
	//	default:
	//		reqModel.ToolChoice = requestTool{
	//			Type: "function",
	//			Function: toolFunc{
	//				Name: g.request.ToolConfig.Name,
	//			},
	//		}
	//	}
	//}

	// Dealing with Output Schema
	if g.request.OutputSchema != nil {
		reqModel.Format = g.request.OutputSchema
	}

	// Dealing with Prompt Messages
	//var hasPayload bool
	messages := []genRequestMessage{}
	for _, c := range conversation {
		message := genRequestMessage{
			Role:    string(c.Role),
			Content: c.Text,
		}
		if c.Payload != nil && prompt.MIMEImages[c.Payload.Mime] {
			//hasPayload = true
			message.Images = append(message.Images, c.Payload.Data)
			// Ollama really need content here to awnser the question it seems. Multiple messages does not seem to work
			// Merge payload with next message...
			message.Content = ""
		}
		messages = append(messages, message)
	}

	//if hasPayload {
	//	tbd := []int{}
	//	for i := 0; i < len(messages); i++ {
	//		if messages[i].Image != "" {
	//			if i+1 < len(messages) {
	//				messages[i].Content = messages[i+1].Content
	//				tbd = append(tbd, i+1)
	//				i++
	//			}
	//		}
	//	}
	//	for _, i := range tbd {
	//		messages = append(messages[:i], messages[i+1:]...)
	//	}
	//}

	reqModel.Messages = messages

	body, err := json.Marshal(reqModel)
	if err != nil {
		return nil, fmt.Errorf("could not marshal open ai request, %w", err)
	}

	u, err := url.JoinPath(g.ollama.uri, "/api/chat")
	if err != nil {
		return nil, fmt.Errorf("could not join url, %w", err)
	}

	ctx := g.request.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("could not create openai request, %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	reqc := atomic.AddInt64(&requestNo, 1)
	g.ollama.log("[gen] request",
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

	var respModel genResponse
	err = json.Unmarshal(body, &respModel)
	if err != nil {
		return nil, fmt.Errorf("could not decode openai response, %w", err)
	}

	res := &gen.Response{
		Texts: []string{respModel.Message.Content},
		Metadata: models.Metadata{
			Model:        g.request.Model.FQN(),
			InputTokens:  respModel.PromptEvalCount,
			OutputTokens: respModel.EvalCount,
			TotalTokens:  respModel.PromptEvalCount + respModel.EvalCount,
		},
	}
	//for _, c := range respModel.Choices {
	//	message := c.Message
	//	if len(message.ToolCalls) == 0 { // Not Tools
	//		res.Texts = append(res.Texts, c.Message.Content)
	//	}
	//
	//	if len(message.ToolCalls) > 0 { // Tool calls
	//		for _, t := range message.ToolCalls {
	//			res.Tools = append(res.Tools, tools.Call{
	//				Name:     t.Function.Name,
	//				Argument: t.Function.Arguments,
	//				Ref:      toolBelt[t.Function.Name],
	//			})
	//		}
	//	}
	//}

	g.ollama.log("[gen] response",
		"request", reqc,
		"model", g.request.Model.FQN(),
		"token-input", res.Metadata.InputTokens,
		"token-output", res.Metadata.OutputTokens,
		"token-total", res.Metadata.TotalTokens,
	)

	return res, nil
}