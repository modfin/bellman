package bfcl

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

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

type Instance struct {
	Replay *replay.Replay
	Tracer *tracer.Tracer
	timer  *time.Timer
	mu     sync.Mutex
}

type Cache struct {
	Instances map[string]*Instance
	mu        sync.Mutex
}

func NewCache() *Cache {
	return &Cache{
		Instances: make(map[string]*Instance),
	}
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

	// ensure cache instance, replay cache and tracer
	i := c.ensureCache(req)

	// stop finish timer once working and defer reset
	i.mu.Lock()
	i.timer.Stop()
	i.mu.Unlock()

	defer func() {
		i.mu.Lock()
		i.timer.Reset(15 * time.Second)
		i.mu.Unlock()
	}()

	i.replayGenerateBFCL(w, req, nil)
}

// replayGenerateBFCL is the replay and generate loop for benchmarking
func (i *Instance) replayGenerateBFCL(w http.ResponseWriter, req BenchmarkRequest, previousGen *gen.Response) {
	bellmanUrl := os.Getenv("BELLMAN_URL")
	bellmanToken := os.Getenv("BELLMAN_TOKEN")
	client := bellman.New(bellmanUrl, bellman.Key{Name: "bfcl", Token: bellmanToken})

	bellmanTools := utils.ParseJsonSchemaTools(req.Tools, req.EnablePTC)

	// add trailing user messages to toolman conversation
	toolmanConversation := i.addNewUserConversation(req)

	if !req.EnablePTC {
		// add benchmark responses to tool calls
		toolmanConversation = i.appendResponseConversation(toolmanConversation, req, nil)
	}

	model, err := gen.ToModel(req.Model)
	if err != nil {
		i.Tracer.TraceError(i.Tracer.RootSpan, err)
		log.Fatalf("to model error: %e", err)
	}

	// Execution replay! - run if new tool responses and PTC enabled
	if req.EnablePTC {
		if len(req.NewToolResponses) > 0 {
			for _, m := range req.NewToolResponses {
				// add response to cache and execute reply again (until execution finishes)
				i.Replay.AddResponse(replay.CallRecord{
					ToolName: m.ToolName,
					Result:   m.Content,
				})
				// trace code execution
				toolResponse := prompt.AsToolResponse(m.ToolID, m.ToolName, m.Content)
				i.Tracer.TraceExec(toolResponse)
			}
		}
		// while there are scripts to run, replay them
		for i.Replay.IsPending() {
			resp, toolResponse := i.executionReplay(bellmanTools, toolmanConversation, previousGen)
			if resp != nil {
				w.Header().Set("Content-Type", "application/json")
				if err = json.NewEncoder(w).Encode(resp); err != nil {
					log.Printf("Failed to write response to client: %v", err)
				}
				return
			}
			// Add response to toolman conversation
			toolmanConversation = i.appendResponseConversation(toolmanConversation, req, toolResponse)
		}
	}

	// trace llm call start (if not recording already)
	if i.Tracer.ChatSpan.Span == nil || !i.Tracer.ChatSpan.IsRecording() {
		i.Tracer.Trace(prompt.AsUser("..."), toolmanConversation)
	}

	if req.EnablePTC {
		req.SystemPrompt = req.SystemPrompt + `
# Rules

- Call ONLY the Tool Functions needed. Return ALL results directly.
- NO logic: no if/else, no loops, no try/catch, no data transformation, no maths.
- NO defensive coding: assume all calls succeed.
- One var per Function call. Return them all in a single object.

`
	}

	llm := client.Generator().Model(model).
		System(req.SystemPrompt).
		SetTools(bellmanTools...) //.Temperature(req.Temperature)

	if req.EnablePTC {
		llm, err = llm.ActivatePTC(ptc.JavaScript)
		if err != nil {
			log.Printf("warning: %e", err)
		}
	}

	// prompt with retry (bfcl restarts on every test...)
	maxRetries := 10
	var res *gen.Response
	for retry := 0; retry <= maxRetries; retry++ {
		start := time.Now()
		res, err = llm.Prompt(toolmanConversation...)
		duration := time.Since(start)
		fmt.Printf("prompt duration: %v ms\n", duration.Milliseconds())

		if err == nil {
			break
		}

		if retry >= maxRetries {
			log.Printf("Prompt Error: %+v\n", err)
			i.Tracer.TraceError(i.Tracer.ChatSpan, err)

			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		backoff := time.Duration(1<<retry) * time.Second
		log.Printf("Prompt Error: %+v. Retrying in %v...\n", err, backoff)
		time.Sleep(backoff)
	}

	// log token usage
	logExecution(res)

	// get tool call or text response, and add PTC scripts to cache
	toolmanCalls, bfclCalls, bfclToolIDs, err := i.getToolCalls(res)
	if err != nil {
		log.Printf("error getting prompts: %v", err)
		i.Tracer.TraceError(i.Tracer.ChatSpan, err)

		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	toolmanConversation = append(toolmanConversation, toolmanCalls...)

	// trace tool calls
	for _, call := range toolmanCalls {
		i.Tracer.Trace(call, toolmanCalls)
	}

	// If PTC enabled, and we get to this point:
	// If assistant: respond
	// else: might as well restart (replay+llm) --> this will loop replay to extract calls and prompt llm until done (assistant)
	if req.EnablePTC && !res.IsText() {
		req.NewToolResponses = nil
		req.ToolmanHistory = toolmanConversation
		i.replayGenerateBFCL(w, req, res)
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
func (i *Instance) getToolCalls(res *gen.Response) ([]prompt.Prompt, []ExtractedCall, []string, error) {
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
			i.Replay.AddScript(replay.Script{
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
			return nil, nil, nil, err
		}
		bfclCalls = append(bfclCalls, call)
		bfclToolIDs = append(bfclToolIDs, tool.ID)
	}

	return toolmanCalls, bfclCalls, bfclToolIDs, nil
}

// executionReplay runs execution replay and returns bench response or tool response
func (i *Instance) executionReplay(bellmanTools []tools.Tool, toolmanConversation []prompt.Prompt, genResponse *gen.Response) (*BenchmarkResponse, *prompt.Prompt) {
	result := i.Replay.ExecutionReplay(bellmanTools)
	if result.Error != nil {
		i.Tracer.TraceError(i.Tracer.ChatSpan, result.Error)
		log.Fatalf("execution replay error: %+v", result.Error)
	}

	// record --> bench tool call
	if result.Record != nil {
		call := recordToBFCLCall(result.Record)

		// trace code execution
		jsonBytes, err := json.Marshal(result.Record.Argument)
		if err != nil {
			log.Printf("error: error marshaling arguments: %+v, args: %+v\n", err, result.Record.Argument)
		}
		toolCall := prompt.AsToolCall(result.ToolID, result.Record.ToolName, jsonBytes)
		i.Tracer.TraceExec(toolCall)

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
		return nil, fmt.Errorf("toolman to bfcl call error: %w", err)
	}

	call := ExtractedCall{
		tool.Name: argsMap,
	}
	return call, nil
}

// ensureCache clears cache on new test (only user messages inbound)
func (c *Cache) ensureCache(req BenchmarkRequest) *Instance {
	c.mu.Lock()

	i, ok := c.Instances[req.TestID]
	if !ok {
		i = &Instance{
			Replay: replay.NewReplay(),
			Tracer: tracer.NewTracer("BFCL"),
		}
		i.timer = time.AfterFunc(15*time.Second, func() {
			c.finish(req.TestID)
		})
		c.Instances[req.TestID] = i
	} else {
		i.timer.Reset(15 * time.Second)
	}
	c.mu.Unlock()

	i.mu.Lock()
	defer i.mu.Unlock()

	reset := true
	for _, m := range req.Messages {
		if m.Role != "user" {
			reset = false
			break
		}
	}
	if reset {
		i.Replay.Clear()
		i.Tracer.NewTrace(tracer.TracerRequest{
			Model:          req.Model,
			ToolmanHistory: req.ToolmanHistory,
			Tools:          req.Tools,
			SystemPrompt:   req.SystemPrompt,
			TestID:         req.TestID,
		})
	}

	return i
}

func (c *Cache) finish(testID string) {
	c.mu.Lock()
	i, ok := c.Instances[testID]
	if ok {
		delete(c.Instances, testID)
	}
	c.mu.Unlock()

	if ok {
		i.mu.Lock()
		defer i.mu.Unlock()

		i.timer.Stop()
		i.Tracer.SendTrace(true)
		i.Replay.Clear()
	}
}

// addNewUserConversation adds incoming user messages to toolman conversation
func (i *Instance) addNewUserConversation(req BenchmarkRequest) []prompt.Prompt {
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
				i.Tracer.NewTurn()
				userPrompt := prompt.AsUser(m.Content)
				i.Tracer.Trace(userPrompt, toolmanHistory)
				toolmanHistory = append(toolmanHistory, userPrompt)
			}
		}
	}
	return toolmanHistory
}

// appendResponseConversation rebuilds the toolman conversation to add new tool response (after corresponding tool call)
func (i *Instance) appendResponseConversation(toolmanHistory []prompt.Prompt, req BenchmarkRequest, response *prompt.Prompt) []prompt.Prompt {
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
				i.Tracer.Trace(*response, nil)
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
						i.Tracer.Trace(responsePrompt, nil)
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
	fmt.Printf("[Token Stats] Request: %d / %d | Global Total: %d / %d\n",
		inputTokens, outputTokens,
		atomic.LoadUint64(&GlobalInputTokens), atomic.LoadUint64(&GlobalOutputTokens))
}
