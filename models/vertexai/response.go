package vertexai

import (
	"encoding/json"
	"fmt"
	"github.com/modfin/bellman"
	"github.com/modfin/bellman/tools"
)

type response struct {
	llm   geminiResponse
	tools []tools.Tool
}

type functionCall struct {
	Name string `json:"name"`
	Arg  any    `json:"args"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Role  string `json:"role"`
			Parts []struct {
				Text         string       `json:"text"`
				FunctionCall functionCall `json:"functionCall"`
			} `json:"parts"`
		} `json:"content"`
		FinishReason  string `json:"finishReason"`
		SafetyRatings []struct {
			Category         string  `json:"category"`
			Probability      string  `json:"probability"`
			ProbabilityScore float64 `json:"probabilityScore"`
			Severity         string  `json:"severity"`
			SeverityScore    float64 `json:"severityScore"`
		} `json:"safetyRatings"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
}

func (r *response) AsText() (string, error) {

	candidates := r.llm.Candidates

	if len(candidates) == 0 {
		return "", fmt.Errorf("no choices in response")
	}
	if len(candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no choices in response")
	}
	return candidates[0].Content.Parts[0].Text, nil
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

func (r *response) AsTools() ([]bellman.ToolCallback, error) {

	candidates := r.llm.Candidates

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}
	if len(candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no tool call in response")
	}

	var res []bellman.ToolCallback

	belt := map[string]*tools.Tool{}
	for _, t := range r.tools {
		belt[t.Name] = &t
	}

	for _, c := range candidates[0].Content.Parts {

		arg, err := json.Marshal(c.FunctionCall.Arg)
		if err != nil {
			return nil, err
		}

		res = append(res, bellman.ToolCallback{

			Name:     c.FunctionCall.Name,
			Argument: string(arg),
			Local:    belt[c.FunctionCall.Name],
		})
	}

	return res, nil
}

func (r *response) Unmarshal(ref any) error {
	text, err := r.AsText()
	if err != nil {
		return err
	}

	return json.Unmarshal([]byte(text), ref)
}
