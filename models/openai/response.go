package openai

import (
	"encoding/json"
	"fmt"
	"github.com/modfin/bellman"
	"github.com/modfin/bellman/schema"
	"github.com/modfin/bellman/tools"
)

type respone struct {
	tools []tools.Tool
	llm   openaiResponse
}

type openaiResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int    `json:"created"`
	Model   string `json:"model"`
	Usage   struct {
		PromptTokens            int `json:"prompt_tokens"`
		CompletionTokens        int `json:"completion_tokens"`
		TotalTokens             int `json:"total_tokens"`
		CompletionTokensDetails struct {
			ReasoningTokens          int `json:"reasoning_tokens"`
			AcceptedPredictionTokens int `json:"accepted_prediction_tokens"`
			RejectedPredictionTokens int `json:"rejected_prediction_tokens"`
		} `json:"completion_tokens_details"`
	} `json:"usage"`
	Choices []struct {
		Message struct {
			Role      string             `json:"role"`
			Content   string             `json:"content"`
			ToolCalls []responseToolCall `json:"tool_calls"`
		} `json:"message"`
		Logprobs     any    `json:"logprobs"`
		FinishReason string `json:"finish_reason"`
		Index        int    `json:"index"`
	} `json:"choices"`
}

func (r *respone) Eval() (err error) {
	callbacks, err := r.AsTools()
	if err != nil {
		return err
	}

	count := 0
	for _, tool := range callbacks {
		for _, t := range r.tools {
			if t.Name != tool.Name {
				continue
			}
			if t.Callback == nil {
				return fmt.Errorf("tool %s has no callback", tool)
			}
			count++
			_, err = t.Callback(tool.Argument)
			if err != nil {
				return fmt.Errorf("tool %s failed: %w", tool, err)
			}
			break
		}

	}
	if count != len(callbacks) {
		return fmt.Errorf("not all callbacks were evaluated")
	}
	return nil
}

func (r *respone) AsTools() ([]bellman.ToolCallback, error) {
	if len(r.llm.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}
	if len(r.llm.Choices[0].Message.ToolCalls) == 0 {
		return nil, fmt.Errorf("no tool call in response")
	}

	belt := map[string]*tools.Tool{}
	for _, t := range r.tools {
		belt[t.Name] = &t
	}
	var res []bellman.ToolCallback

	for _, c := range r.llm.Choices[0].Message.ToolCalls {
		res = append(res, bellman.ToolCallback{
			Name:     c.Function.Name,
			Argument: c.Function.Arguments,
			Local:    belt[c.Function.Name],
		})
	}

	return res, nil
}

func (r *respone) AsText() (string, error) {
	if len(r.llm.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}
	return r.llm.Choices[0].Message.Content, nil
}
func (r *respone) Unmarshal(ref any) error {
	text, err := r.AsText()
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(text), ref)
}

type responseToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Arguments string `json:"arguments"`
		Name      string `json:"name"`
	} `json:"function"`
}

type toolFunc struct {
	Name        string       `json:"name"`
	Parameters  *schema.JSON `json:"parameters,omitempty"`
	Description string       `json:"description,omitempty"`
	Strict      bool         `json:"strict,omitempty"`
}
