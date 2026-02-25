package cfb

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/dop251/goja"
	"github.com/modfin/bellman"
	"github.com/modfin/bellman/models/gen"
	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/tools"
	"github.com/modfin/bellman/tools/ptc"
	"github.com/modfin/bellman/tools/ptc/bench/replay"
	"github.com/modfin/bellman/tools/ptc/bench/utils"
)

type BenchmarkRequest struct {
	Model       string        `json:"model"`
	Messages    []Message     `json:"messages"`
	Tools       []interface{} `json:"tools"`
	Temperature float64       `json:"temperature"`
	//SystemPrompt   string          `json:"system_prompt"`
	//ToolChoice string `json:"tool_choice"`
	MaxTokens      int             `json:"max_tokens"`
	EnablePTC      bool            `json:"enable_ptc"`
	ToolmanHistory []prompt.Prompt `json:"toolman_history"`
	ToolmanCalls   []prompt.Prompt `json:"toolman_calls"`
}

type Message struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls"`
	ToolName  string     `json:"tool_name"`
	ToolID    string     `json:"tool_call_id"`
}

type BenchmarkResponse struct {
	Completion     ChatCompletionResponse `json:"completion"`
	ToolmanHistory []prompt.Prompt        `json:"toolman_history"`
	ToolmanCalls   []prompt.Prompt        `json:"toolman_calls"`
}

type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

type Choice struct {
	Index        int             `json:"index"`
	Message      ResponseMessage `json:"message"`
	FinishReason string          `json:"finish_reason"`
}

type ResponseMessage struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"` // Nullable in JSON, empty string in Go
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // Vital: This is a stringified JSON blob
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ExtractedCall: The structure of a single tool call found during execution
type ExtractedCall map[string]map[string]interface{}

// ExecutionResult holds both the calls found and the final return value of the script
type ExecutionResult struct {
	Calls []ToolCall `json:"tool_calls"`
	Error error      `json:"error"`
}

type Replay struct {
	Cache *replay.ReplayCache
}

var (
	GlobalInputTokens  uint64
	GlobalOutputTokens uint64
)

func (replay *Replay) HandleGenerateCFB(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	//PrintRequest(r) // Debug requests

	var req BenchmarkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	bellmanUrl := os.Getenv("BELLMAN_URL")
	bellmanToken := os.Getenv("BELLMAN_TOKEN")
	client := bellman.New(bellmanUrl, bellman.Key{Name: "cfb", Token: bellmanToken})

	bfclTools := utils.ParseJsonSchemaTools(req.Tools, req.EnablePTC)

	toolmanHistory := req.ToolmanHistory

	// count toolman user messages
	toolmanUserCount := 0
	for _, p := range toolmanHistory {
		if p.Role == prompt.UserRole {
			toolmanUserCount++
		}
	}
	// add trailing messages from BFCL
	bfclUserCount := 0
	for _, m := range req.Messages {
		switch m.Role {
		case "user":
			// only add new user messages from bfcl (not in toolman hist.)
			bfclUserCount++
			if bfclUserCount > toolmanUserCount {
				toolmanHistory = append(toolmanHistory, prompt.AsUser(m.Content))
			}
		}
	}

	// Add tool response after call!
	var rebuiltHistory []prompt.Prompt
	for i, p := range toolmanHistory {
		switch p.Role {
		case prompt.ToolCallRole:
			rebuiltHistory = append(rebuiltHistory, p)
			// find all corresponding tool results and concatenate
			var concatenatedReturns []string
			for j := 0; j < len(req.Messages); j++ {
				if req.Messages[j].Role == "tool" && req.Messages[j].ToolID == p.ToolCall.ToolCallID {
					concatenatedReturns = append(concatenatedReturns, fmt.Sprintf("Function '%s' result: %s.", req.Messages[j].ToolName, req.Messages[j].Content))
				}
			}
			// add JS runtime errors to tool response
			if len(toolmanHistory) > i+1 {
				nextPrompt := toolmanHistory[i+1]
				if nextPrompt.Role == prompt.ToolResponseRole && nextPrompt.ToolResponse.ToolCallID == p.ToolCall.ToolCallID {
					concatenatedReturns = append(concatenatedReturns, nextPrompt.ToolResponse.Response)
				}
			}
			rebuiltHistory = append(rebuiltHistory, prompt.AsToolResponse(p.ToolCall.ToolCallID, p.ToolCall.Name, strings.Join(concatenatedReturns, "\n")))
		case prompt.UserRole:
			rebuiltHistory = append(rebuiltHistory, p)
			//case prompt.AssistantRole: // <-- assistant should only come from toolman response, not added here!
			//	rebuiltHistory = append(rebuiltHistory, p)
		}
	}

	model, err := gen.ToModel(req.Model)
	if err != nil {
		log.Fatalf("error: %e", err)
	}
	//model = openai.GenModel_gpt4_1_mini_250414

	// set variable warning; cant inject tool response into goja var
	systemPrompt := ""
	if req.EnablePTC { // TODO: this seems dumb, but need to rewrite system prompt otherwise...
		systemPrompt = "WARNING: You are running a benchmark, which means tool function outputs are NOT assigned to variables. " +
			"You must assume that variables can be reset between turns without warning. " +
			"If you receive new information from a tool function call, you MUST set the variable in the top of the script to make sure you are able to use it." +
			"This means you need to disregard any variable statements or assumptions listed below."
	}

	llm := client.Generator().Model(model).
		System(""). // TODO: check if system prompt available?
		SetTools(bfclTools...).
		System(systemPrompt).
		SetPTCLanguage(tools.JavaScript).
		Temperature(req.Temperature)

	res, err := llm.Prompt(rebuiltHistory...)
	if err != nil {
		log.Printf("Prompt Error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// EXTRACT TOKENS & UPDATE GLOBAL COUNTERS
	inputTokens := res.Metadata.InputTokens
	outputTokens := res.Metadata.OutputTokens

	// Thread-safe increment
	atomic.AddUint64(&GlobalInputTokens, uint64(inputTokens))
	atomic.AddUint64(&GlobalOutputTokens, uint64(outputTokens))

	// Log the running total to the console
	log.Printf("[Token Stats] Request: %d / %d | Global Total: %d / %d",
		inputTokens, outputTokens,
		atomic.LoadUint64(&GlobalInputTokens), atomic.LoadUint64(&GlobalOutputTokens))

	// extract individual new tool calls for bfcl + toolman
	extractedCalls, toolmanCalls, err := GetToolCalls(res, bfclTools)
	if err != nil {
		log.Fatalf("error: %e", err)
	}

	// add new toolman calls to conversation history
	toolmanHistory = append(toolmanHistory, toolmanCalls...)

	content := ""
	if res.IsText() {
		if content, err = res.AsText(); err != nil {
			log.Printf("error: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	finishReason := "stop"
	if res.IsTools() {
		finishReason = "tool_calls"
	}

	completion := ChatCompletionResponse{
		ID:      "chatcmpl-123", // Important: fill with mock data! (for completion parsing in cfb)
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model.String(),
		Choices: []Choice{{
			Index: 0,
			Message: ResponseMessage{
				Role:      "assistant",
				Content:   content,
				ToolCalls: extractedCalls,
			},
			FinishReason: finishReason,
		},
		},
		Usage: Usage{
			PromptTokens:     inputTokens,
			CompletionTokens: outputTokens,
			TotalTokens:      inputTokens + outputTokens,
		},
	}

	resp := BenchmarkResponse{
		Completion:     completion,
		ToolmanHistory: toolmanHistory,
		ToolmanCalls:   toolmanCalls,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func PrintRequest(r *http.Request) {
	bodyBytes, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, bodyBytes, "", "  "); err != nil {
		fmt.Printf("Received Raw Body: %s\n", string(bodyBytes))
	} else {
		fmt.Printf("Received Pretty JSON:\n%s\n", prettyJSON.String())
	}
}

// GetToolCalls extracts calls in the Ground Truth format: [{"func": {"arg": val}}]
func GetToolCalls(res *gen.Response, availableTools []tools.Tool) ([]ToolCall, []prompt.Prompt, error) {
	// CFB
	var calls []ToolCall
	// Toolman
	var toolCalls []prompt.Prompt

	if !res.IsTools() { // --> res.IsText()
		text, err := res.AsText()
		if err != nil {
			log.Fatalf("error: %e", err)
		}
		assistant := []prompt.Prompt{prompt.AsAssistant(text)}
		return calls, assistant, nil
	}

	for _, tool := range res.Tools {
		// --- PTC / Code Execution ---
		if tool.Name == ptc.CodeExecutionToolName {
			var codeArgs struct {
				Code string `json:"code"`
			}
			// Unmarshal the 'argument' string/bytes to get the JS code
			if err := json.Unmarshal(tool.Argument, &codeArgs); err == nil {
				// Run the Extractor
				execResult := ExecuteAndExtract(codeArgs.Code, availableTools)
				// Append all calls found in the JS code (with correct ID)
				for _, c := range execResult.Calls {
					c.ID = tool.ID
				}
				calls = append(calls, execResult.Calls...)

				// add toolman call + ID & check for JS execution errors!
				toolCalls = append(toolCalls, prompt.AsToolCall(tool.ID, tool.Name, tool.Argument))
				//toolIDs = append(toolIDs, tool.ID)
				if execResult.Error != nil {
					toolCalls = append(toolCalls, prompt.AsToolResponse(tool.ID, tool.Name, execResult.Error.Error())) // will not be added to bfcl tool calls!
					//toolIDs = append(toolIDs, tool.ID) // <-- don't think this is needed... only for returned bfcl tools
				}
			} else {
				fmt.Printf("Warning: error unmarshalling code_execution argument: %e\n", err)
			}
			continue
		}

		// --- Standard Tool Call ---
		//Map it to the same structure: {"func_name": {"arg": val}}
		//var argsMap map[string]interface{}
		//
		//// Try unmarshalling argument. If it fails (rare), we skip args or make empty map
		//if err := json.Unmarshal(tool.Argument, &argsMap); err != nil {
		//	fmt.Printf("Warning: Failed to unmarshal args for %s: %v\n", tool.Name, err)
		//	argsMap = make(map[string]interface{})
		//}

		toolCalls = append(toolCalls, prompt.AsToolCall(tool.ID, tool.Name, tool.Argument))

		// Construct the entry
		entry := ToolCall{
			ID:   tool.ID,
			Type: "function",
			Function: ToolCallFunction{
				Name:      tool.Name,
				Arguments: string(tool.Argument),
			},
		}
		calls = append(calls, entry)
	}

	return calls, toolCalls, nil
}

func ExecuteAndExtract(jsCode string, availableTools []tools.Tool) *ExecutionResult {
	// GLOBAL SAFETY: Recover from any internal Panic
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Critical Panic in Interpreter: %v\n", r)
		}
	}()

	vm := goja.New()
	var capturedCalls []ToolCall

	// TIMEOUT SAFETY: Prevent infinite loops (e.g. while(true))
	// Interrupt execution after 500ms.
	timer := time.AfterFunc(500*time.Millisecond, func() {
		vm.Interrupt("timeout")
	})
	defer timer.Stop()

	// POLYFILLS: Prevent ReferenceErrors for common globals
	// LLMs often treat 'console' and 'print' as standard.
	dummyFunc := func(call goja.FunctionCall) goja.Value { return vm.ToValue(nil) }
	vm.Set("print", dummyFunc)

	console := vm.NewObject()
	console.Set("log", dummyFunc)
	console.Set("error", dummyFunc)
	console.Set("warn", dummyFunc)
	vm.Set("console", console)

	for _, tool := range availableTools {
		tName := tool.Name

		// INTERCEPTOR: Runs when JS calls a tool
		interceptor := func(call goja.FunctionCall) goja.Value {
			argsMap := make(map[string]interface{})

			// ROBUST ARG PARSING
			if len(call.Arguments) > 0 {
				// Export() converts goja.Value to Go interface{}
				firstArg := call.Arguments[0].Export()

				// Scenario A: Standard (First arg is a Dictionary/Object)
				if obj, ok := firstArg.(map[string]interface{}); ok {
					for k, v := range obj {
						argsMap[k] = v
					}
				} else {
					// Scenario B: Positional Args or Non-Object
					// We capture them with generic keys
					argsMap["__arg_0__"] = firstArg
				}

				// Capture remaining positional arguments if any
				for i := 1; i < len(call.Arguments); i++ {
					key := fmt.Sprintf("__arg_%d__", i)
					argsMap[key] = call.Arguments[i].Export()
				}
			}

			// parse json into string
			argsBytes, err := json.Marshal(argsMap)
			argsStr := "{}"
			if err == nil {
				argsStr = string(argsBytes)
			} else {
				fmt.Printf("[Warning] Failed to marshal args for %s: %v\n", tName, err)
			}

			// Record the call
			capturedCalls = append(capturedCalls, ToolCall{
				ID:   "", // should be replaced later
				Type: "function",
				Function: ToolCallFunction{
					Name:      tName,
					Arguments: argsStr,
				},
			})

			// Return generic mock to keep script running
			mock := vm.NewObject()
			mock.Set("status", "success")
			mock.Set("success", true)
			mock.Set("error", nil)

			return mock
		}
		vm.Set(tName, interceptor)
	}

	_, err := vm.RunString(jsCode)

	// GRACEFUL FAILURE
	// If we crash (e.g. syntax error), we STILL return whatever calls we captured.
	if err != nil {
		// Check if it was just our timeout
		var evalErr *goja.InterruptedError
		if !errors.As(err, &evalErr) {
			// If it's a real runtime error, just log it.
			// We DO NOT return the error to the caller, because we want the partial results.
			err = fmt.Errorf("javascript runtime error: %s", err)
			fmt.Printf("[warning] JS Runtime Error: %v\n", err)
		}
	}

	//fmt.Printf("________ Code:\n%v\n", jsCode)

	return &ExecutionResult{
		Calls: capturedCalls,
		Error: err,
	}
}
