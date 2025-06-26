package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/modfin/bellman/models"
	"github.com/modfin/bellman/models/gen"
	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/schema"
	"github.com/modfin/bellman/tools"
	"sync"
)

// Run will prompt until the llm responds with no tool calls, or until maxDepth is reached. Unless Output is already
// set, it will be set by using schema.From on the expected result struct. Does not work with gemini as of 2025-02-17.
func Run[T any](maxDepth int, parallelism int, g *gen.Generator, prompts ...prompt.Prompt) (*Result[T], error) {
	var result T
	_, resultIsString := any(result).(string)
	if g.Request.OutputSchema == nil && !resultIsString {
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
			// Check if T is string type and handle directly
			if resultIsString {
				text, err := resp.AsText()
				if err != nil {
					return nil, fmt.Errorf("could not get text response: %w, at depth %d", err, i)
				}
				// Convert string to T (which we know is string) using unsafe casting
				result = any(text).(T)
			} else {
				err = resp.Unmarshal(&result)
				if err != nil {
					return nil, fmt.Errorf("could not unmarshal text response: %w, at depth %d", err, i)
				}
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

		// Pre-validate all callbacks before execution
		for _, callback := range callbacks {
			if callback.Ref == nil {
				return nil, fmt.Errorf("tool %s not found in local setup", callback.Name)
			}
			if callback.Ref.Function == nil {
				return nil, fmt.Errorf("tool %s has no callback function attached", callback.Name)
			}
		}

		var callbackResults []callbackResult
		if parallelism <= 1 {
			callbackResults = executeCallbacksSequential(g.Request.Context, callbacks)
		} else {
			callbackResults = executeCallbacksParallel(g.Request.Context, callbacks, parallelism)
		}

		// Process results and check for errors
		for _, cbResult := range callbackResults {
			callback := callbacks[cbResult.Index]
			prompts = append(prompts, prompt.AsToolCall(callback.ID, callback.Name, callback.Argument))

			if cbResult.Error != nil {
				return nil, fmt.Errorf("tool %s failed: %w, arg: %s", cbResult.Name, cbResult.Error, callback.Argument)
			}

			prompts = append(prompts, prompt.AsToolResponse(cbResult.ID, cbResult.Name, cbResult.Response))
		}

	}
	return nil, fmt.Errorf("max depth %d reached", maxDepth)
}

const customResultCalculatedTool = "__return_result_tool__"

// RunWithToolsOnly will prompt until the llm responds with a certain tool call. Prefer to use the Run function above,
// but gemini does not support the above function (requiring tools and structured output), so use this one instead for those models.
func RunWithToolsOnly[T any](maxDepth int, parallelism int, g *gen.Generator, prompts ...prompt.Prompt) (*Result[T], error) {
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

		// Pre-validate all callbacks before execution
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
			if callback.Ref == nil {
				return nil, fmt.Errorf("tool %s not found in local setup", callback.Name)
			}
			if callback.Ref.Function == nil {
				return nil, fmt.Errorf("tool %s has no callback function attached", callback.Name)
			}
		}

		var callbackResults []callbackResult
		if parallelism <= 1 {
			callbackResults = executeCallbacksSequential(g.Request.Context, callbacks)
		} else {
			callbackResults = executeCallbacksParallel(g.Request.Context, callbacks, parallelism)
		}

		// Process results and check for errors
		for _, cbResult := range callbackResults {
			callback := callbacks[cbResult.Index]
			prompts = append(prompts, prompt.AsToolCall(callback.ID, callback.Name, callback.Argument))

			if cbResult.Error != nil {
				return nil, fmt.Errorf("tool %s failed: %w, arg: %s", cbResult.Name, cbResult.Error, callback.Argument)
			}

			prompts = append(prompts, prompt.AsToolResponse(cbResult.ID, cbResult.Name, cbResult.Response))
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

// callbackResult holds the result of a single callback execution
type callbackResult struct {
	Index    int
	ID       string
	Name     string
	Response string
	Error    error
}

// executeCallbacksSequential executes callbacks one by one (original behavior)
func executeCallbacksSequential(ctx context.Context, callbacks []tools.Call) []callbackResult {
	results := make([]callbackResult, len(callbacks))

	for i, callback := range callbacks {
		response, err := callback.Ref.Function(ctx, callback)
		results[i] = callbackResult{
			Index:    i,
			ID:       callback.ID,
			Name:     callback.Name,
			Response: response,
			Error:    err,
		}
	}

	return results
}

// executeCallbacksParallel executes callbacks in parallel with limited concurrency
func executeCallbacksParallel(ctx context.Context, callbacks []tools.Call, parallelism int) []callbackResult {
	numCallbacks := len(callbacks)
	results := make([]callbackResult, numCallbacks)

	// Use a semaphore to limit concurrency
	semaphore := make(chan struct{}, parallelism)
	var wg sync.WaitGroup

	for i, callback := range callbacks {
		wg.Add(1)
		go func(index int, cb tools.Call) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			response, err := cb.Ref.Function(ctx, cb)
			results[index] = callbackResult{
				Index:    index,
				ID:       cb.ID,
				Name:     cb.Name,
				Response: response,
				Error:    err,
			}
		}(i, callback)
	}

	wg.Wait()
	return results
}
