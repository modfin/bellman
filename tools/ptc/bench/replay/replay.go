package replay

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/dop251/goja"
	"github.com/modfin/bellman/tools"
)

type ReplayCache struct {
	mu      sync.RWMutex
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

type ReplayResult struct {
	Record *CallRecord
	Output string
	ToolID string
	Error  error
}

// NewCache creates a new cache
func NewCache() *ReplayCache {
	return &ReplayCache{
		record: []CallRecord{},
	}
}

// AddResponse adds a tool response to the cache
func (r *ReplayCache) AddResponse(record CallRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.record = append(r.record, record)
}

// AddScript adds a script to the cache
func (r *ReplayCache) AddScript(script Script) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if script.Code == "" { // TODO handle/prevent with guardrail
		log.Printf("empty code script!")
	}
	r.Scripts = append(r.Scripts, script)
}

// Clear wipes the cache on demand
func (r *ReplayCache) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.record = []CallRecord{}
	r.Scripts = []Script{}
	r.Replay()
}

// Replay resets the cursor index (to replay execution)
func (r *ReplayCache) Replay() {
	r.Cursor = 0
}

// IsPending returns true if there is a pending script to run
func (r *ReplayCache) IsPending() bool {
	for _, s := range r.Scripts {
		if !s.Done {
			return true
		}
	}
	return false
}

// ExecutionReplay reruns code script until finish or error --> let llm decide next step (return response or fix error)
// Returns a recorded (tool) call, code execution result, or error
func (r *ReplayCache) ExecutionReplay(tools []tools.Tool) ReplayResult {

	// Destroy state! Create a new VM for every single replay (prevent unexpected errors)
	vm := goja.New()
	r.Replay()

	//// fix: Inject deterministic overrides to prevent the LLM from branching randomly TODO is this needed?!
	//vm.RunString(`
	//		Math.random = function() { return 0.42; };
	//		Date.now = function() { return 1708682816000; };
	//	`)

	// Inject our cached tools into the VM, and add interrupt on new tool calls
	for _, t := range tools {
		err := interceptCall(vm, r, t)
		if err != nil {
			return ReplayResult{Error: err}
		}
	}

	// Run the next code script
	for i, s := range r.Scripts {
		fmt.Printf("____ running script: %s\n", s.Code)
		val, err := vm.RunString(s.Code) // TODO: guardrail against empty code?
		if err != nil {
			// script crash, or intentional interrupt?
			var jsErr *goja.InterruptedError
			if errors.As(err, &jsErr) {
				if record, isYield := jsErr.Value().(*CallRecord); isYield {
					// new tool call!
					fmt.Printf("Yielding to Benchmark for: %s...\n", record.ToolName)

					return ReplayResult{Record: record, ToolID: s.ToolID}
				}
			}
			// Important: return JS error as JSON so LLM can see it
			return ReplayResult{Output: fmt.Sprintf("error: %q", err.Error())}
		}

		// If we reach here, the script finished successfully without yielding!
		// 'val' is the final output of the script, and cache.Records holds the exact sequence of tool calls.
		// return response if first script completion, and set done
		if !s.Done {
			r.Scripts[i].Done = true // use index to access actual object
			// Safely convert final Goja value to JSON string!
			var jsonBytes []byte
			if val == nil || goja.IsUndefined(val) {
				jsonBytes = []byte("null")
			} else {
				jsonBytes, err = json.Marshal(val.Export())
				if err != nil {
					return ReplayResult{Error: err}
				}
			}
			fmt.Printf("=== script output: %s\n", string(jsonBytes))
			return ReplayResult{Output: string(jsonBytes), ToolID: s.ToolID}
		}
	}
	log.Printf("already replayed scripts") // TODO should this be allowed?
	return ReplayResult{}
}

func interceptCall(vm *goja.Runtime, cache *ReplayCache, tool tools.Tool) error {
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
