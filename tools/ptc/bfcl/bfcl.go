package bfcl

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync/atomic"

	"github.com/modfin/bellman"
	"github.com/modfin/bellman/models/gen"
	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/tools"
	"github.com/modfin/bellman/tools/ptc"
	"github.com/modfin/bellman/tools/ptc/bench/replay"
	"github.com/modfin/bellman/tools/ptc/bench/utils"
)

type BenchmarkRequest struct {
	Model            string          `json:"bellman_model"`
	Messages         []Message       `json:"messages"`
	NewToolResponses []Message       `json:"new_tool_responses"`
	ToolmanHistory   []prompt.Prompt `json:"toolman_history"`
	Tools            []interface{}   `json:"tools"`
	Temperature      float64         `json:"temperature"`
	SystemPrompt     string          `json:"system_prompt"`
	EnablePTC        bool            `json:"enable_ptc"`
	ReplayCode       string          `json:"replay_code"`
}

type Message struct {
	Role     string `json:"role"`
	Content  string `json:"content"`
	ToolName string `json:"tool_name"`
	ToolID   string `json:"tool_call_id"`
}

type BenchmarkResponse struct {
	ToolCalls      []ExtractedCall `json:"tool_calls"`
	ToolCallIDs    []string        `json:"tool_call_ids"`
	ToolmanHistory []prompt.Prompt `json:"toolman_history"`
	Content        string          `json:"content"`       // Any thought/text
	InputTokens    int             `json:"input_tokens"`  // Added for tracking
	OutputTokens   int             `json:"output_tokens"` // Added for tracking
}

// ExtractedCall is a bfcl tool call to be returned
type ExtractedCall map[string]map[string]interface{}

type Replay struct {
	*replay.ReplayCache
}

var (
	GlobalInputTokens  uint64
	GlobalOutputTokens uint64
)

func (replayCache *Replay) HandleGenerateBFCL(w http.ResponseWriter, r *http.Request) {
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
	client := bellman.New(bellmanUrl, bellman.Key{Name: "bfcl", Token: bellmanToken})

	bellmanTools := utils.ParseJsonSchemaTools(req.Tools, req.EnablePTC)

	// Rebuild toolman conversation with new messages
	toolmanConversation := rebuildToolmanConversation(req.ToolmanHistory, req, nil)

	// clear cache on new test (only user messages)
	reset := true
	for _, m := range req.Messages {
		if m.Role != "user" {
			reset = false
			break
		}
	}
	if reset {
		fmt.Printf("clearing cache\n")
		replayCache.Clear()
	}

	// Execution replay!
	// rerun code script until finish or error --> let llm decide next step (return response or fix error)
	// Run replay if new tool responses, and PTC enabled
	// PTC: if there are new tool responses, add them to the cache and run replay
	if len(req.NewToolResponses) > 0 && req.EnablePTC {
		for _, m := range req.NewToolResponses {
			// add response to cache and execute reply again (until execution finishes)
			fmt.Printf("adding result: %s --> %s\n", m.ToolName, m.Content)
			replayCache.AddResponse(replay.CallRecord{
				ToolName: m.ToolName,
				Result:   m.Content,
			})
		}
		// while there are scripts to run, replay them
		for replayCache.IsPending() {
			result := replayCache.ExecutionReplay(bellmanTools)
			if result.Error != nil {
				log.Fatalf("error: %e", result.Error)
			}

			// record --> bench tool call
			if result.Record != nil {
				call := recordToBFCLCall(result.Record)

				// return call, only 1 at a time
				resp := BenchmarkResponse{
					ToolCalls:      []ExtractedCall{call},
					ToolCallIDs:    []string{result.ToolID},
					ToolmanHistory: req.ToolmanHistory,
					InputTokens:    0,
					OutputTokens:   0,
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
				return
			}

			// execution result --> toolman response
			response := prompt.AsToolResponse(result.ToolID, ptc.CodeExecutionToolName, result.Output)

			// Rebuild toolman conversation with new messages
			toolmanConversation = rebuildToolmanConversation(toolmanConversation, req, []prompt.Prompt{response}) // TODO needs to be response list?
		}
	}

	model, err := gen.ToModel(req.Model)
	if err != nil {
		log.Fatalf("error: %e", err)
	}
	//model = openai.GenModel_gpt5_mini_latest

	// remove bfcl system prompt for PTC - misleading! TODO remove for non-PTC as well?
	if req.EnablePTC {
		req.SystemPrompt = ""
	}

	llm := client.Generator().Model(model).
		System(req.SystemPrompt).
		SetTools(bellmanTools...).
		SetPTCLanguage(tools.JavaScript) //.Temperature(req.Temperature)

	res, err := llm.Prompt(toolmanConversation...)
	if err != nil {
		log.Printf("Prompt Error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// log token usage
	logExecution(res)

	// get tool call or text response, and add PTC scripts to cache
	toolmanPrompts, toolCalls, err := replayCache.getToolmanPrompts(res)
	if err != nil {
		log.Printf("error getting prompts: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	toolmanHistory := append(toolmanConversation, toolmanPrompts...)

	// if there is pending scripts, run them to extract next tool call/response
	for replayCache.IsPending() {
		result := replayCache.ExecutionReplay(bellmanTools)
		if result.Error != nil {
			log.Fatalf("error: %e", result.Error)
		}

		// record --> bench tool call
		if result.Record != nil {
			call := recordToBFCLCall(result.Record)

			// return call, only 1 at a time
			resp := BenchmarkResponse{
				ToolCalls:      []ExtractedCall{call},
				ToolCallIDs:    []string{result.ToolID},
				ToolmanHistory: toolmanHistory, // use latest history
				InputTokens:    res.Metadata.InputTokens,
				OutputTokens:   res.Metadata.OutputTokens,
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}

		// TODO: this should never happen!? - unless empty code (no tools)
		// execution result --> toolman response
		response := prompt.AsToolResponse(result.ToolID, ptc.CodeExecutionToolName, result.Output)
		//toolmanHistory = append(toolmanHistory, response)

		// Rebuild toolman conversation with new messages
		toolmanHistory = rebuildToolmanConversation(toolmanHistory, req, []prompt.Prompt{response}) // TODO needs to be response list?
	}

	//// extract individual new tool calls for bfcl + toolman
	var extractedCalls []ExtractedCall
	var extractedToolIDs []string
	for _, c := range toolCalls {
		call, err := toolmanToBFCLCall(c)
		if err != nil {
			log.Fatalf("error: %e", err)
		}
		extractedCalls = append(extractedCalls, call)
		extractedToolIDs = append(extractedToolIDs, c.ID)
	}

	resp := BenchmarkResponse{
		ToolCalls:      extractedCalls, // TODO assistant calls seem to become added as new tool results!?!
		ToolCallIDs:    extractedToolIDs,
		ToolmanHistory: toolmanHistory,
		//Content:        "Tool calls generated", // TODO <-- is this used in bfcl?
		InputTokens:  res.Metadata.InputTokens,
		OutputTokens: res.Metadata.OutputTokens,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// getToolmanPrompts extracts prompts from response
func (replayCache *Replay) getToolmanPrompts(res *gen.Response) ([]prompt.Prompt, []tools.Call, error) {
	var calls []tools.Call

	// response is assistant text
	if !res.IsTools() { // --> res.IsText()
		text, err := res.AsText()
		if err != nil {
			return nil, nil, err
		}
		assistant := []prompt.Prompt{prompt.AsAssistant(text)}
		return assistant, calls, nil
	}

	// response is tool calls
	var toolCalls []prompt.Prompt
	for _, tool := range res.Tools {
		// PTC Tool Call
		if tool.Name == ptc.CodeExecutionToolName {
			// Unmarshal the 'argument' string/bytes to get the JS code
			var codeArgs struct {
				Code string `json:"code"`
			}
			err := json.Unmarshal(tool.Argument, &codeArgs)
			if err != nil {
				return nil, nil, err
			}

			// add script to replay cache
			replayCache.AddScript(replay.Script{
				Code:   codeArgs.Code,
				Done:   false,
				ToolID: tool.ID,
			})
			//record, result, err := replayCache.ExecutionReplay()
			toolCalls = append(toolCalls, prompt.AsToolCall(tool.ID, tool.Name, tool.Argument))
			calls = append(calls, tool)
			continue
		}

		// Standard Tool Call
		toolCalls = append(toolCalls, prompt.AsToolCall(tool.ID, tool.Name, tool.Argument))
		calls = append(calls, tool)
	}

	return toolCalls, calls, nil
}

// recordToBFCLCall converts replay record to bfcl tool call
func recordToBFCLCall(record *replay.CallRecord) ExtractedCall {
	call := ExtractedCall{
		record.ToolName: record.Argument,
	}
	return call
}

// toolmanToBFCLCall converts toolman call to bfcl tool call
func toolmanToBFCLCall(tool tools.Call) (ExtractedCall, error) {
	var argsMap map[string]interface{}
	if err := json.Unmarshal(tool.Argument, &argsMap); err != nil {
		return nil, err
	}

	call := ExtractedCall{
		tool.Name: argsMap,
	}
	return call, nil
}

// rebuildToolmanConversation rebuilds the conversation with history and new benchmark messages
func rebuildToolmanConversation(toolmanHistory []prompt.Prompt, req BenchmarkRequest, responses []prompt.Prompt) []prompt.Prompt {
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
	var rebuiltConversation []prompt.Prompt
	for _, p := range toolmanHistory {
		switch p.Role {
		case prompt.ToolCallRole:
			rebuiltConversation = append(rebuiltConversation, p)

			// add corresponding tool call response (only add once)
			found := false
			for _, r := range responses {
				if r.ToolResponse.ToolCallID == p.ToolCall.ToolCallID {
					rebuiltConversation = append(rebuiltConversation, r)
					found = true
					break
				}
			}
			// if not found: and already in history, add it
			if !found {
				for _, h := range toolmanHistory {
					if h.Role == prompt.ToolResponseRole && p.ToolCall.ToolCallID == h.ToolResponse.ToolCallID {
						rebuiltConversation = append(rebuiltConversation, h)
						found = true
						break
					}
				}
			}
			// if not found: add from previous tool responses (and non-PTC)
			if !found {
				for _, m := range req.Messages {
					if m.Role == "tool_response" && m.ToolID == p.ToolCall.ToolCallID {
						rebuiltConversation = append(rebuiltConversation, prompt.AsToolResponse(m.ToolID, m.ToolName, m.Content))
						break
					}
				}
			}
		case prompt.UserRole:
			rebuiltConversation = append(rebuiltConversation, p)
		case prompt.AssistantRole: // <-- assistant should only come from toolman llm response
			rebuiltConversation = append(rebuiltConversation, p)
		}
	}

	return rebuiltConversation
}

func logExecution(res *gen.Response) {
	// extract tokens and update global counters
	inputTokens := res.Metadata.InputTokens
	outputTokens := res.Metadata.OutputTokens

	// Thread-safe increment
	atomic.AddUint64(&GlobalInputTokens, uint64(inputTokens))
	atomic.AddUint64(&GlobalOutputTokens, uint64(outputTokens))

	// Log the running total to the console
	log.Printf("[Token Stats] Request: %d / %d | Global Total: %d / %d",
		inputTokens, outputTokens,
		atomic.LoadUint64(&GlobalInputTokens), atomic.LoadUint64(&GlobalOutputTokens))
}
