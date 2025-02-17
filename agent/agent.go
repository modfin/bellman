package agent

import (
	"fmt"
	"github.com/modfin/bellman/models/gen"
	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/schema"
)

// Run will prompt until the llm responds with no tool calls, or until maxDepth is reached. Unless Output is already
// set, it will be set by using schema.From on the expected result struct. Does not work with gemini as of 2025-02-17.
func Run[T any](maxDepth int, g *gen.Generator, prompts ...prompt.Prompt) (*Result[T], error) {
	if g.Request.OutputSchema == nil {
		var result T
		g = g.Output(schema.From(result))
	}

	for i := 0; i < maxDepth; i++ {
		resp, err := g.Prompt(prompts...)
		if err != nil {
			return nil, fmt.Errorf("failed to prompt: %w, at depth %d", err, i)
		}

		if !resp.IsTools() {
			var result T
			err = resp.Unmarshal(&result)
			if err != nil {
				return nil, fmt.Errorf("could not unmarshal text response: %w, at depth %d", err, i)
			}
			return &Result[T]{
				Prompts: prompts,
				Result:  result,
				Depth:   i,
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
			toolFuncResponse, err := callback.Ref.Function(callback.Argument)
			if err != nil {
				return nil, fmt.Errorf("tool %s failed: %w, arg: %s", callback.Name, err, callback.Argument)
			}
			prompts = append(prompts, prompt.AsToolResponse(callback.ID, callback.Name, toolFuncResponse))
		}

	}
	return nil, fmt.Errorf("max depth %d reached", maxDepth)

}

type Result[T any] struct {
	Prompts []prompt.Prompt
	Result  T
	Depth   int
}
