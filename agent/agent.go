package agent

import (
	"encoding/json"
	"fmt"
	"github.com/modfin/bellman/models"
	"github.com/modfin/bellman/models/gen"
	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/schema"
	"github.com/modfin/bellman/tools"
)

// Run will prompt until the llm responds with no tool calls, or until maxDepth is reached. Unless Output is already
// set, it will be set by using schema.From on the expected result struct. Does not work with gemini as of 2025-02-17.
func Run[T any](maxDepth int, g *gen.Generator, prompts ...prompt.Prompt) (*Result[T], error) {
	if g.Request.OutputSchema == nil {
		var result T
		g = g.Output(schema.From(result))
	}

	promptMetadata := models.Metadata{Model: g.Request.Model.Name}
	for i := 0; i < maxDepth; i++ {
		resp, err := g.Prompt(prompts...)
		if err != nil {
			return nil, fmt.Errorf("failed to prompt: %w, at depth %d", err, i)
		}
		promptMetadata.InputTokens += resp.Metadata.InputTokens
		promptMetadata.OutputTokens += resp.Metadata.OutputTokens
		promptMetadata.TotalTokens += resp.Metadata.TotalTokens

		if !resp.IsTools() {
			var result T
			err = resp.Unmarshal(&result)
			if err != nil {
				return nil, fmt.Errorf("could not unmarshal text response: %w, at depth %d", err, i)
			}
			return &Result[T]{
				Prompts:  prompts,
				Result:   result,
				Metadata: promptMetadata,
				Depth:    i,
			}, nil
		}

		callbacks, err := resp.AsTools()
		if err != nil {
			return nil, fmt.Errorf("failed to get tools: %w, at depth %d", err, i)
		}

		for _, callback := range callbacks {
			prompts = append(prompts, prompt.AsToolCall(callback.ID, callback.Name, callback.Argument))
			if callback.Ref == nil {
				return nil, fmt.Errorf("tool %s not found in local setup", callback.Name)
			}
			if callback.Ref.Function == nil {
				return nil, fmt.Errorf("tool %s has no callback function attached", callback.Name)
			}
			toolFuncResponse, err := callback.Ref.Function(g.Request.Context, callback.Argument)
			if err != nil {
				return nil, fmt.Errorf("tool %s failed: %w, arg: %s", callback.Name, err, callback.Argument)
			}
			prompts = append(prompts, prompt.AsToolResponse(callback.ID, callback.Name, toolFuncResponse))
		}

	}
	return nil, fmt.Errorf("max depth %d reached", maxDepth)
}

const customResultCalculatedTool = "__return_result_tool__"

// RunWithToolsOnly will prompt until the llm responds with a certain tool call. Prefer to use the Run function above,
// but gemini does not support the above function (requiring tools and structured output), so use this one instead for those models.
func RunWithToolsOnly[T any](maxDepth int, g *gen.Generator, prompts ...prompt.Prompt) (*Result[T], error) {
	if g.Request.OutputSchema != nil {
		g = g.Output(nil)
	}

	var newTools []tools.Tool
	for _, t := range g.Tools() {
		if t.Name == customResultCalculatedTool {
			continue
		}
		newTools = append(newTools, t)
	}
	g = g.SetTools(newTools...)

	var result T
	g = g.AddTools(tools.Tool{
		Name:           customResultCalculatedTool,
		Description:    "Return the final results to the user",
		ArgumentSchema: schema.From(result),
	})
	g = g.SetToolConfig(tools.RequiredTool)

	promptMetadata := models.Metadata{Model: g.Request.Model.Name}
	for i := 0; i < maxDepth; i++ {
		resp, err := g.Prompt(prompts...)
		if err != nil {
			return nil, fmt.Errorf("failed to prompt: %w, at depth %d", err, i)
		}
		promptMetadata.InputTokens += resp.Metadata.InputTokens
		promptMetadata.OutputTokens += resp.Metadata.OutputTokens
		promptMetadata.TotalTokens += resp.Metadata.TotalTokens

		callbacks, err := resp.AsTools()
		if err != nil {
			return nil, fmt.Errorf("failed to get tools: %w, at depth %d", err, i)
		}

		for _, callback := range callbacks {
			if callback.Name == customResultCalculatedTool {
				var finalResult T
				err = json.Unmarshal(callback.Argument, &finalResult)
				if err != nil {
					return nil, fmt.Errorf("could not unmarshal final result: %w, at depth %d", err, i)
				}
				return &Result[T]{
					Prompts:  prompts,
					Result:   finalResult,
					Metadata: promptMetadata,
					Depth:    i,
				}, nil
			}
			prompts = append(prompts, prompt.AsToolCall(callback.ID, callback.Name, callback.Argument))
			if callback.Ref == nil {
				return nil, fmt.Errorf("tool %s not found in local setup", callback.Name)
			}
			if callback.Ref.Function == nil {
				return nil, fmt.Errorf("tool %s has no callback function attached", callback.Name)
			}
			toolFuncResponse, err := callback.Ref.Function(g.Request.Context, callback.Argument)
			if err != nil {
				return nil, fmt.Errorf("tool %s failed: %w, arg: %s", callback.Name, err, callback.Argument)
			}
			prompts = append(prompts, prompt.AsToolResponse(callback.ID, callback.Name, toolFuncResponse))
		}
	}
	return nil, fmt.Errorf("max depth %d reached", maxDepth)
}

type Result[T any] struct {
	Prompts  []prompt.Prompt
	Result   T
	Metadata models.Metadata
	Depth    int
}
