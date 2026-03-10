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
	"github.com/modfin/bellman/tools/ptc/bench/tracer"
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
	TestID           string          `json:"test_entry_id"`
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

type Cache struct {
	Replay *replay.Replay
	Tracer *tracer.Tracer
}

var (
	GlobalInputTokens  uint64
	GlobalOutputTokens uint64
)

// HandleGenerateBFCL is the handler for the BFCL benchmark
func (c *Cache) HandleGenerateBFCL(w http.ResponseWriter, r *http.Request) {
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
		c.ensureCache(req)
	}

	c.replayGenerateBFCL(w, req, nil)
}

// replayGenerateBFCL is the replay and generate loop for benchmarking
func (c *Cache) replayGenerateBFCL(w http.ResponseWriter, req BenchmarkRequest, previousGen *gen.Response) {
	bellmanUrl := os.Getenv("BELLMAN_URL")
	bellmanToken := os.Getenv("BELLMAN_TOKEN")
	client := bellman.New(bellmanUrl, bellman.Key{Name: "bfcl", Token: bellmanToken})

	bellmanTools := utils.ParseJsonSchemaTools(req.Tools, req.EnablePTC)

	// add trailing user messages to toolman conversation
	toolmanConversation := c.addNewUserConversation(req)

	if !req.EnablePTC {
		// add benchmark responses to tool calls
		toolmanConversation = c.appendResponseConversation(toolmanConversation, req, nil)
	}

	model, err := gen.ToModel(req.Model)
	if err != nil {
		log.Fatalf("error: %e", err)
	}
	//model = openai.GenModel_gpt5_mini_latest

	// Execution replay! - run if new tool responses and PTC enabled
	if req.EnablePTC {
		if len(req.NewToolResponses) > 0 {
			for _, m := range req.NewToolResponses {
				// add response to cache and execute reply again (until execution finishes)
				fmt.Printf("adding result: %s --> %s\n", m.ToolName, m.Content)
				c.Replay.AddResponse(replay.CallRecord{
					ToolName: m.ToolName,
					Result:   m.Content,
				})
				// trace code execution
				toolResponse := prompt.AsToolResponse(m.ToolID, m.ToolName, m.Content)
				c.Tracer.TraceExec(toolResponse)
			}
		}
		// while there are scripts to run, replay them
		for c.Replay.IsPending() {
			resp, toolResponse := c.executionReplay(bellmanTools, toolmanConversation, previousGen)
			if resp != nil {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
				return
			}
			// Add response to toolman conversation
			toolmanConversation = c.appendResponseConversation(toolmanConversation, req, toolResponse)
		}
	}

	// trace llm call start (if not recording already)
	if c.Tracer.ChatSpan.Span == nil || !c.Tracer.ChatSpan.IsRecording() {
		c.Tracer.Trace(prompt.AsUser(""), toolmanConversation)
	}

	llm := client.Generator().Model(model).
		System(req.SystemPrompt).
		SetTools(bellmanTools...) //.Temperature(req.Temperature)

	if req.EnablePTC {
		llm, _ = llm.ActivatePTC(ptc.JavaScript)
	}

	res, err := llm.Prompt(toolmanConversation...)
	if err != nil {
		log.Printf("Prompt Error: %v", err)
		c.Tracer.TraceError(c.Tracer.ChatSpan, err)

		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// log token usage
	logExecution(res)

	// get tool call or text response, and add PTC scripts to cache
	toolmanCalls, bfclCalls, bfclToolIDs, err := c.getToolCalls(res)
	if err != nil {
		log.Printf("error getting prompts: %v", err)
		c.Tracer.TraceError(c.Tracer.ChatSpan, err)

		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	toolmanConversation = append(toolmanConversation, toolmanCalls...)

	// trace tool calls
	for _, call := range toolmanCalls {
		c.Tracer.Trace(call, toolmanCalls)
	}

	// If PTC enabled, and we get to this point:
	// If assistant: respond
	// else: might as well restart (replay+llm) --> this will loop replay to extract calls and prompt llm until done (assistant)
	if req.EnablePTC && !res.IsText() {
		req.NewToolResponses = nil
		req.ToolmanHistory = toolmanConversation
		c.replayGenerateBFCL(w, req, res)
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
func (c *Cache) getToolCalls(res *gen.Response) ([]prompt.Prompt, []ExtractedCall, []string, error) {
	var bfclCalls []ExtractedCall
	var bfclToolIDs []string

	// response is assistant text
	if !res.IsTools() { // --> res.IsText()
		text, err := res.AsText()
		if err != nil {
			return nil, nil, nil, err
		}
		assistant := prompt.AsAssistant(text)
		return []prompt.Prompt{assistant}, nil, nil, nil
	}

	// response is tool calls
	var toolmanCalls []prompt.Prompt
	for _, tool := range res.Tools {
		// PTC Tool Call
		if tool.Name == ptc.PTCToolName {
			// Unmarshal the 'argument' string/bytes to get the JS code
			var codeArgs struct {
				Code string `json:"code"`
			}
			err := json.Unmarshal(tool.Argument, &codeArgs)
			if err != nil {
				return nil, nil, nil, err
			}

			// add script to replay cache
			c.Replay.AddScript(replay.Script{
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
func (c *Cache) executionReplay(bellmanTools []tools.Tool, toolmanConversation []prompt.Prompt, genResponse *gen.Response) (*BenchmarkResponse, *prompt.Prompt) {
	result := c.Replay.ExecutionReplay(bellmanTools)
	if result.Error != nil {
		log.Fatalf("error: %e", result.Error)
	}

	// record --> bench tool call
	if result.Record != nil {
		call := recordToBFCLCall(result.Record)

		// trace code execution
		jsonBytes, err := json.Marshal(result.Record.Argument)
		if err != nil {
			log.Printf("error: Error marshaling arguments: %v\n", err)
		}
		toolCall := prompt.AsToolCall(result.ToolID, result.Record.ToolName, jsonBytes)
		c.Tracer.TraceExec(toolCall)

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
	toolResponse := prompt.AsToolResponse(result.ToolID, ptc.PTCToolName, result.Output)
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
func (c *Cache) ensureCache(req BenchmarkRequest) {
	reset := true
	for _, m := range req.Messages {
		if m.Role != "user" {
			reset = false
			break
		}
	}
	if reset {
		fmt.Printf("clearing cache & new trace\n")
		c.Replay.Clear()
		c.Tracer.NewTrace(tracer.TracerRequest{
			Model:          req.Model,
			ToolmanHistory: req.ToolmanHistory,
			Tools:          req.Tools,
			SystemPrompt:   req.SystemPrompt,
			TestID:         req.TestID,
		})
	}
}

// addNewUserConversation adds incoming user messages to toolman conversation
func (c *Cache) addNewUserConversation(req BenchmarkRequest) []prompt.Prompt {
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
				// update turn index & trace
				c.Tracer.NewTurn()
				userPrompt := prompt.AsUser(m.Content)
				c.Tracer.Trace(userPrompt, toolmanHistory)
				toolmanHistory = append(toolmanHistory, userPrompt)
			}
		}
	}
	return toolmanHistory
}

// appendResponseConversation rebuilds the toolman conversation to add new tool response (after corresponding tool call)
func (c *Cache) appendResponseConversation(toolmanHistory []prompt.Prompt, req BenchmarkRequest, response *prompt.Prompt) []prompt.Prompt {
	// Add tool response after call!
	var rebuiltConversation []prompt.Prompt
	for _, p := range toolmanHistory {
		switch p.Role {
		case prompt.ToolCallRole:
			rebuiltConversation = append(rebuiltConversation, p)

			// add corresponding tool call response (only add once)
			// priority order: response -> toolman history -> request messages
			if response != nil && response.ToolResponse.ToolCallID == p.ToolCall.ToolCallID {
				// trace tool response
				c.Tracer.Trace(*response, nil)
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
						// trace tool response
						responsePrompt := prompt.AsToolResponse(m.ToolID, m.ToolName, m.Content)
						c.Tracer.Trace(responsePrompt, nil)
						rebuiltConversation = append(rebuiltConversation, responsePrompt)
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
