package anthropic

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/modfin/bellman"
	"github.com/modfin/bellman/tools"
)

const respone_output_callback_name = "__bellman__result_callback"

type response struct {
	llm anthropicResponse

	tools []tools.Tool
}

func (r *response) outputCallback() bool {
	// TODO check response
	return len(r.llm.Content) > 0 && r.llm.Content[0].Type == "tool_use" && r.llm.Content[0].Name == respone_output_callback_name
}

func (r *response) AsText() (string, error) {
	if len(r.llm.Content) == 0 {
		return "", errors.New("no content in response")
	}

	if r.outputCallback() {
		b, err := json.Marshal(r.llm.Content[0].Input)
		return string(b), err
	}

	if r.llm.Content[0].Type != "text" {
		return "", errors.New("response is not text")
	}

	return r.llm.Content[0].Text, nil
}

func (r *response) AsTools() ([]bellman.ToolCallback, error) {
	if len(r.llm.Content) == 0 {
		return nil, errors.New("no content in response")
	}

	if r.outputCallback() {
		return nil, fmt.Errorf("response is structured output, use AsText of unmarshal instead")
	}

	var ret []bellman.ToolCallback
	for _, c := range r.llm.Content {
		if c.Type == "tool_use" {
			jsondata, err := json.Marshal(c.Input)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal input: %w", err)
			}
			ret = append(ret, bellman.ToolCallback{
				Name:     c.Name,
				Argument: string(jsondata),
			})
		}
	}
	if len(ret) == 0 {
		return nil, errors.New("no tool use in response")
	}

	return ret, nil
}

func (r *response) Eval() (err error) {
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
			err = t.Callback(tool.Argument)
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

func (r *response) Unmarshal(ref any) error {
	text, err := r.AsText()
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(text), ref)
}

type anthropicResponse struct {
	Content []struct {
		Type  string `json:"type"` // text or tool_use
		Text  string `json:"text"`
		Name  string `json:"name"`
		Input any    `json:"input"`
	} `json:"content"`
	ID           string `json:"id"`
	Model        string `json:"model"`
	Role         string `json:"role"`
	StopReason   string `json:"stop_reason"`
	StopSequence any    `json:"stop_sequence"`
	Type         string `json:"type"`
	Usage        struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}
