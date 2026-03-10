package replay

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/dop251/goja"
	"github.com/modfin/bellman/tools"
	"github.com/modfin/bellman/tools/ptc"
	"github.com/modfin/bellman/tools/ptc/js"
)

type Replay struct {
	mu      sync.Mutex
	record  []CallRecord
	Cursor  int
	Scripts []Script
}

// CallRecord stores the history of executed tools and their benchmark responses
type CallRecord struct {
	ToolName string
	Argument map[string]interface{}
	Result   string
}

// Script represents a code script to run
// also keeps track of which tool ID scripts belongs to
type Script struct {
	Code   string
	Done   bool
	ToolID string
}

type Result struct {
	Record *CallRecord
	Output string
	ToolID string
	Error  error
}

// NewReplay creates a new cache
func NewReplay() *Replay {
	return &Replay{
		record: []CallRecord{},
	}
}

// AddResponse adds a tool response to the cache
func (r *Replay) AddResponse(record CallRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.record = append(r.record, record)
}

// AddScript adds a script to the cache
func (r *Replay) AddScript(script Script) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if script.Code == "" { // TODO handle/prevent with guardrail?
		log.Printf("empty code script!")
	}
	r.Scripts = append(r.Scripts, script)
}

// Clear wipes the cache on demand
func (r *Replay) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.record = []CallRecord{}
	r.Scripts = []Script{}
	r.Replay()
}

// Replay resets the cursor index (to replay execution)
func (r *Replay) Replay() {
	r.Cursor = 0
}

// IsPending returns true if there is a pending script to run
func (r *Replay) IsPending() bool {
	for _, s := range r.Scripts {
		if !s.Done {
			return true
		}
	}
	return false
}

// ExecutionReplay reruns code script until finish or error --> let llm decide next step (return response or fix error)
// Returns a recorded (tool) call, code execution result, or error
// important: if JS errors, let LLM see it (return as string)
func (r *Replay) ExecutionReplay(tools []tools.Tool) Result {

	// Destroy state! Create a new VM for every single replay (prevent unexpected errors)
	runtime := js.NewRuntime(ptc.PTCToolName)
	r.Replay()

	// Inject our cached tools into the VM, and add interrupt on new tool calls
	for _, t := range tools {
		err := interceptCall(runtime.Runtime(), r, t)
		if err != nil {
			return Result{Error: err}
		}
	}

	// Run the next code script
	for i, s := range r.Scripts {
		fmt.Printf("____ running script: %s\n", s.Code)

		res, resErr, err := runtime.Execute(s.Code)
		if err != nil {
			return Result{Error: err}
		}

		// runtime error
		if resErr != nil {
			// intentional interrupt?
			var jsErr *goja.InterruptedError
			if errors.As(resErr, &jsErr) {
				if record, isYield := jsErr.Value().(*CallRecord); isYield {
					// new tool call!
					fmt.Printf("Yielding to Benchmark for: %s...\n", record.ToolName)
					return Result{Record: record, ToolID: s.ToolID}
				}
			}
			// script crash
			if !s.Done {
				r.Scripts[i].Done = true // index to access actual object
				return Result{Output: fmt.Sprintf("error: %q", resErr.Error()), ToolID: s.ToolID}
			}
		}

		// If we reach here, the script finished successfully without yielding!
		// 'val' is the final output of the script, and cache.Records holds the exact sequence of tool calls.
		// return response if first script completion, and set done
		if !s.Done {
			r.Scripts[i].Done = true // use index to access actual object
			fmt.Printf("=== script output: %s\n", res)
			return Result{Output: res, ToolID: s.ToolID}
		}
	}
	log.Printf("already replayed scripts") // TODO should this be allowed?
	return Result{}
}

func interceptCall(vm *goja.Runtime, cache *Replay, tool tools.Tool) error {
	interceptor := func(call goja.FunctionCall) goja.Value {
		// Cache hit: replaying script and already know the answer
		if cache.Cursor < len(cache.record) {
			record := cache.record[cache.Cursor]

			// ensure the script is deterministic!
			if record.ToolName != tool.Name {
				return vm.NewGoError(fmt.Errorf("error: expected %s, got %s (record)", tool.Name, record.ToolName))
			}

			cache.Cursor++

			// if None, null, or empty --> undefined
			if record.Result == "None" || record.Result == "null" || record.Result == "{}" || record.Result == "" {
				// Return a native JavaScript 'undefined' (or goja.Null())
				return goja.Undefined()
			}

			// Parse the cached JSON string back into a native Goja object so the script can use it
			var parsed interface{}
			err := json.Unmarshal([]byte(record.Result), &parsed)
			if err != nil {
				return vm.NewGoError(fmt.Errorf("error: could not unmarshal result from cache: %v", err))
			}
			return vm.ToValue(parsed)
		}

		// Cache miss: new tool call --> interrupt VM
		argsMap := extractArgsMap(call)

		vm.Interrupt(&CallRecord{
			ToolName: tool.Name,
			Argument: argsMap,
		})
		return nil
	}

	err := vm.Set(tool.Name, interceptor)
	if err != nil {
		return err
	}

	return nil
}

func extractArgsMap(call goja.FunctionCall) map[string]interface{} {
	argsMap := make(map[string]interface{})
	if len(call.Arguments) > 0 {
		firstArg := call.Argument(0).Export()

		// Scenario A: Standard PTC (first arg is a dictionary)
		// tool({ "arg": 1 })
		if obj, ok := firstArg.(map[string]interface{}); ok {
			for k, v := range obj {
				argsMap[k] = v
			}
		} else {
			// Scenario B: Hallucinated Positional Args
			// tool("value", 123)
			// capture with generic keys so we don't lose data
			argsMap["__arg_0__"] = firstArg
		}

		// Capture remaining positional arguments if any
		for i := 1; i < len(call.Arguments); i++ {
			fmt.Printf("[Fix] caught a previous js extract error...")
			key := fmt.Sprintf("__arg_%d__", i)
			argsMap[key] = call.Arguments[i].Export()
		}
	}
	return argsMap
}
