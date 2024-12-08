package rag

import (
	"encoding/json"
	"fmt"
	"github.com/modfin/bellman/models/gen"
	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/tools"
)

const respone_output_callback_name = "__bellman__rag_result_callback"

func tool2promt(t tools.Call) prompt.Prompt {

	return prompt.Prompt{
		Role: prompt.Assistant,
		Text: "function call: " + t.Name + " with argument: " + t.Argument,
	}
}

func Run[T any](depth int, g *gen.Generator, prompts ...prompt.Prompt) (*Result[T], error) {

	var zero T

	resultTool := tools.NewTool(
		respone_output_callback_name,
		tools.WithDescription("function is called once the Gen is finished with RAG retrieval and want to return the result to the user"),
		tools.WithArgSchema(zero),
	)

	g = g.AddTools(resultTool)
	g = g.SetToolConfig(tools.RequiredTool)

	for i := 0; i < depth; i++ {

		resp, err := g.Prompt(prompts...)
		if err != nil {
			return nil, fmt.Errorf("failed to prompt: %w, at depth %d", err, i)
		}

		callbacks, err := resp.AsTools()
		if err != nil {
			return nil, fmt.Errorf("failed to get tools: %w, at depth %d", err, i)
		}

		for _, callback := range callbacks {
			prompts = append(prompts, tool2promt(callback))

			if callback.Name == respone_output_callback_name {
				var ret T
				err = json.Unmarshal([]byte(callback.Argument), &ret)
				return &Result[T]{
					Promps: prompts,
					Result: ret,
					Depth:  i,
				}, err
			}

			if callback.Ref == nil {
				return nil, fmt.Errorf("tool %s not found in local setup", callback.Name)
			}
			if callback.Ref.Function == nil {
				return nil, fmt.Errorf("tool %s has no callback function attached", callback.Name)
			}

			respstr, err := callback.Ref.Function(callback.Argument)
			if err != nil {
				return nil, fmt.Errorf("tool %s failed: %w", callback.Name, err)
			}
			prompts = append(prompts, prompt.AsUser("result: "+callback.Name+" => "+respstr))
		}

	}
	return nil, fmt.Errorf("depth, %d, limit reached", depth)

}

type Result[T any] struct {
	Promps []prompt.Prompt
	Result T
	Depth  int
}
