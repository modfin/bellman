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
	Content        string          `json:"content"`
	InputTokens    int             `json:"input_tokens"`
	OutputTokens   int             `json:"output_tokens"`
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

// HandleGenerateBFCL is the handler for the BFCL benchmark
func (replayCache *Replay) HandleGenerateBFCL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req BenchmarkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.EnablePTC {
		// ensure replay cache is ready
		replayCache.ensureCache(req)
	}

	replayCache.replayGenerateBFCL(w, req, nil)
}

// replayGenerateBFCL is the replay and generate loop for benchmarking
func (replayCache *Replay) replayGenerateBFCL(w http.ResponseWriter, req BenchmarkRequest, previousGen *gen.Response) {
	bellmanUrl := os.Getenv("BELLMAN_URL")
	bellmanToken := os.Getenv("BELLMAN_TOKEN")
	client := bellman.New(bellmanUrl, bellman.Key{Name: "bfcl", Token: bellmanToken})

	bellmanTools := utils.ParseJsonSchemaTools(req.Tools, req.EnablePTC)

	// add trailing user messages to toolman conversation
	toolmanConversation := addTrailingUserConversation(req)

	if !req.EnablePTC {
		// add benchmark responses to tool calls
		toolmanConversation = appendResponseConversation(toolmanConversation, req, nil)
	}

	// Execution replay! - run if new tool responses and PTC enabled
	if req.EnablePTC {
		if len(req.NewToolResponses) > 0 {
			for _, m := range req.NewToolResponses {
				// add response to cache and execute reply again (until execution finishes)
				fmt.Printf("adding result: %s --> %s\n", m.ToolName, m.Content)
				replayCache.AddResponse(replay.CallRecord{
					ToolName: m.ToolName,
					Result:   m.Content,
				})
			}
		}
		// while there are scripts to run, replay them
		for replayCache.IsPending() {
			resp, toolResponse := replayCache.executionReplay(bellmanTools, toolmanConversation, previousGen)
			if resp != nil {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
				return
			}
			// Add response to toolman conversation
			toolmanConversation = appendResponseConversation(toolmanConversation, req, toolResponse)
		}
	}

	model, err := gen.ToModel(req.Model)
	if err != nil {
		log.Fatalf("error: %e", err)
	}
	//model = openai.GenModel_gpt5_mini_latest

	// remove bfcl system prompt for PTC - misleading!
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
	toolmanCalls, bfclCalls, bfclToolIDs, err := replayCache.getToolCalls(res)
	if err != nil {
		log.Printf("error getting prompts: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	toolmanConversation = append(toolmanConversation, toolmanCalls...)

	// If PTC enabled, and we get to this point:
	// If assistant: respond
	// else: might as well restart (replay+llm) --> this will loop replay to extract calls and prompt llm until done (assistant)
	if req.EnablePTC && !res.IsText() {
		req.NewToolResponses = nil
		req.ToolmanHistory = toolmanConversation
		replayCache.replayGenerateBFCL(w, req, res)
		return
	}

	// return assistant regular tool calls to bfcl (non-ptc)
	resp := BenchmarkResponse{
		ToolCalls:      bfclCalls,
		ToolCallIDs:    bfclToolIDs,
		ToolmanHistory: toolmanConversation,
		InputTokens:    res.Metadata.InputTokens,
		OutputTokens:   res.Metadata.OutputTokens,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// getToolCalls extracts prompts from response
func (replayCache *Replay) getToolCalls(res *gen.Response) ([]prompt.Prompt, []ExtractedCall, []string, error) {
	var bfclCalls []ExtractedCall
	var bfclToolIDs []string

	// response is assistant text
	if !res.IsTools() { // --> res.IsText()
		text, err := res.AsText()
		if err != nil {
			return nil, nil, nil, err
		}
		assistant := []prompt.Prompt{prompt.AsAssistant(text)}
		return assistant, nil, nil, nil
	}

	// response is tool calls
	var toolmanCalls []prompt.Prompt
	for _, tool := range res.Tools {
		// PTC Tool Call
		if tool.Name == ptc.CodeExecutionToolName {
			// Unmarshal the 'argument' string/bytes to get the JS code
			var codeArgs struct {
				Code string `json:"code"`
			}
			err := json.Unmarshal(tool.Argument, &codeArgs)
			if err != nil {
				return nil, nil, nil, err
			}

			// add script to replay cache
			replayCache.AddScript(replay.Script{
				Code:   codeArgs.Code,
				Done:   false,
				ToolID: tool.ID,
			})

			toolmanCalls = append(toolmanCalls, prompt.AsToolCall(tool.ID, tool.Name, tool.Argument))
			continue
		}

		// Standard Tool Call
		toolmanCalls = append(toolmanCalls, prompt.AsToolCall(tool.ID, tool.Name, tool.Argument))
		call, err := toolmanToBFCLCall(tool)
		if err != nil {
			log.Fatalf("error: %e", err)
		}
		bfclCalls = append(bfclCalls, call)
		bfclToolIDs = append(bfclToolIDs, tool.ID)
	}

	return toolmanCalls, bfclCalls, bfclToolIDs, nil
}

// executionReplay runs execution replay and returns bench response or tool response
func (replayCache *Replay) executionReplay(bellmanTools []tools.Tool, toolmanConversation []prompt.Prompt, genResponse *gen.Response) (*BenchmarkResponse, *prompt.Prompt) {
	result := replayCache.ExecutionReplay(bellmanTools)
	if result.Error != nil {
		log.Fatalf("error: %e", result.Error)
	}

	// record --> bench tool call
	if result.Record != nil {
		call := recordToBFCLCall(result.Record)

		inputTokens := 0
		outputTokens := 0
		// set token count if llm response was generated
		if genResponse != nil {
			inputTokens = genResponse.Metadata.InputTokens
			outputTokens = genResponse.Metadata.OutputTokens
		}

		// return call, only 1 at a time
		resp := BenchmarkResponse{
			ToolCalls:      []ExtractedCall{call},
			ToolCallIDs:    []string{result.ToolID},
			ToolmanHistory: toolmanConversation,
			InputTokens:    inputTokens,
			OutputTokens:   outputTokens,
		}

		return &resp, nil
	}

	// execution result --> toolman response
	toolResponse := prompt.AsToolResponse(result.ToolID, ptc.CodeExecutionToolName, result.Output)
	return nil, &toolResponse
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

// ensureCache clears cache on new test (only user messages inbound)
func (replayCache *Replay) ensureCache(req BenchmarkRequest) {
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
}

// addTrailingUserConversation adds incoming user messages to toolman conversation
func addTrailingUserConversation(req BenchmarkRequest) []prompt.Prompt {
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
	return toolmanHistory
}

// appendResponseConversation rebuilds the toolman conversation to add new tool response (after corresponding tool call)
func appendResponseConversation(toolmanHistory []prompt.Prompt, req BenchmarkRequest, response *prompt.Prompt) []prompt.Prompt {
	// Add tool response after call!
	var rebuiltConversation []prompt.Prompt
	for _, p := range toolmanHistory {
		switch p.Role {
		case prompt.ToolCallRole:
			rebuiltConversation = append(rebuiltConversation, p)

			// add corresponding tool call response (only add once)
			// priority order: response -> toolman history -> request messages
			if response != nil && response.ToolResponse.ToolCallID == p.ToolCall.ToolCallID {
				rebuiltConversation = append(rebuiltConversation, *response)
				break
			}
			found := false
			for _, h := range toolmanHistory {
				if h.Role == prompt.ToolResponseRole && p.ToolCall.ToolCallID == h.ToolResponse.ToolCallID {
					rebuiltConversation = append(rebuiltConversation, h)
					found = true
					break
				}
			}
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
		case prompt.AssistantRole:
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
