package cfb

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
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
	Model            string          `json:"model"`
	Messages         []Message       `json:"messages"`
	NewToolResponses []Message       `json:"new_tool_responses"`
	ToolmanHistory   []prompt.Prompt `json:"toolman_history"`
	Tools            []interface{}   `json:"tools"`
	Temperature      float64         `json:"temperature"`
	SystemPrompt     string          `json:"system_prompt"`
	EnablePTC        bool            `json:"enable_ptc"`
	TestID           string          `json:"test_id"`
}

type Message struct {
	Role     string `json:"role"`
	Content  string `json:"content"`
	ToolName string `json:"tool_name"`
	ToolID   string `json:"tool_call_id"`
}

type BenchmarkResponse struct {
	Completion     ChatCompletionResponse `json:"completion"`
	ToolmanHistory []prompt.Prompt        `json:"toolman_history"`
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
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ExtractedCall is a cfb tool call to be returned
type ExtractedCall map[string]map[string]interface{}

type Instance struct {
	Replay  *replay.Replay
	Tracer  *tracer.Tracer
	timer   *time.Timer
	mu      sync.Mutex
	retries int
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

// HandleGenerateCFB is the handler for the CFB benchmark
func (c *Cache) HandleGenerateCFB(w http.ResponseWriter, r *http.Request) {
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
		i.timer.Reset(10 * time.Minute)
		i.mu.Unlock()
	}()

	i.replayGenerateCFB(w, req, nil)
}

// replayGenerateCFB is the replay and generate loop for benchmarking
func (i *Instance) replayGenerateCFB(w http.ResponseWriter, req BenchmarkRequest, previousGen *gen.Response) {
	bellmanUrl := os.Getenv("BELLMAN_URL")
	bellmanToken := os.Getenv("BELLMAN_TOKEN")
	client := bellman.New(bellmanUrl, bellman.Key{Name: "cfb", Token: bellmanToken})

	bellmanTools := utils.ParseJsonSchemaTools(req.Tools, req.EnablePTC)

	model, err := gen.ToModel(req.Model)
	if err != nil {
		i.Tracer.TraceError(i.Tracer.RootSpan, err)
		log.Fatalf("error: %e", err)
	}

	// add trailing user messages to toolman conversation
	toolmanConversation := i.addNewUserConversation(req)

	if !req.EnablePTC {
		// add benchmark responses to tool calls
		toolmanConversation = i.appendResponseConversation(toolmanConversation, req, nil)
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
		if len(req.NewToolResponses) > 0 && !i.Replay.IsPending() {
			log.Printf("?????????")

		}
		// while there are scripts to run, replay them
		for i.Replay.IsPending() {
			resp, toolResponse := i.executionReplay(bellmanTools, toolmanConversation, previousGen, model)
			if resp != nil {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
				return
			}
			// Add response to toolman conversation
			toolmanConversation = i.appendResponseConversation(toolmanConversation, req, toolResponse)
		}
	}

	llm := client.Generator().Model(model).
		System(req.SystemPrompt).
		SetTools(bellmanTools...) //.Temperature(req.Temperature)

	if req.EnablePTC {
		llm, _ = llm.ActivatePTC(ptc.JavaScript)
	}

	// prompt with retry (cfb restarts on every test...)
	maxRetries := 5
	var res *gen.Response
	var metrics *tracer.Metrics
	for {
		// trace llm call start (if not recording already)
		if i.Tracer.ChatSpan.Span == nil || !i.Tracer.ChatSpan.IsRecording() {
			i.Tracer.Trace(prompt.AsUser(""), toolmanConversation, nil)
		}

		start := time.Now()
		res, err = llm.Prompt(toolmanConversation...)
		duration := time.Since(start)
		fmt.Printf("prompt duration: %v ms\n", duration.Milliseconds())

		if res != nil {
			metrics = &tracer.Metrics{
				InputTokens:    res.Metadata.InputTokens,
				OutputTokens:   res.Metadata.OutputTokens,
				ThinkingTokens: res.Metadata.ThinkingTokens,
			}
		}

		if err == nil {
			break
		}

		if i.retries >= maxRetries {
			log.Printf("Prompt Error: %+v\n", err)
			i.Tracer.TraceError(i.Tracer.ChatSpan, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// trace error as assistant
		i.Tracer.Trace(prompt.AsAssistant(err.Error()), toolmanConversation, nil)

		// update retry counter
		i.retries++

		if strings.Contains(err.Error(), "unexpected status code 403") {
			// return on 403 error (llm provider fire wall)
			completion := ChatCompletionResponse{
				ID:      "chatcmpl-123", // Important: fill with mock data! (for completion parsing in cfb)
				Object:  "chat.completion",
				Created: time.Now().Unix(),
				Model:   model.String(),
				Choices: []Choice{{
					Index: 0,
					Message: ResponseMessage{
						Role:      "assistant",
						Content:   "error",
						ToolCalls: nil,
					},
					FinishReason: "stop",
				},
				},
				Usage: Usage{
					PromptTokens:     0,
					CompletionTokens: 0,
					TotalTokens:      0,
				},
			}

			resp := BenchmarkResponse{
				Completion:     completion,
				ToolmanHistory: toolmanConversation,
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}

		backoff := time.Duration(1<<i.retries) * time.Second
		log.Printf("Prompt Error: %+v. Retrying in %v...\n", err, backoff)
		time.Sleep(backoff)
	}

	// log token usage
	logExecution(res)

	// get tool call or text response, and add PTC scripts to cache
	toolmanCalls, cfbCalls, err := i.getToolCalls(res)
	if err != nil {
		log.Printf("error getting prompts: %v", err)
		i.Tracer.TraceError(i.Tracer.ChatSpan, err)

		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	toolmanConversation = append(toolmanConversation, toolmanCalls...)

	// trace tool calls
	for _, call := range toolmanCalls {
		if res.IsTools() {
			i.Tracer.Trace(call, toolmanCalls, metrics)
		}
	}

	// If PTC enabled, and we get to this point:
	// If assistant: respond
	// else: might as well restart (replay+llm) --> this will loop replay to extract calls and prompt llm until done (assistant)
	if req.EnablePTC && !res.IsText() {
		req.NewToolResponses = nil
		req.ToolmanHistory = toolmanConversation
		i.replayGenerateCFB(w, req, res)
		return
	}

	// return assistant or regular tool calls to cfb (non-ptc)
	content := ""
	if res.IsText() {
		if content, err = res.AsText(); err != nil {
			log.Printf("error: %v", err)
			i.Tracer.TraceError(i.Tracer.ChatSpan, err)

			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		//i.Tracer.Trace(prompt.AsAssistant(content), toolmanConversation, metrics) TODO needed?
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
				ToolCalls: cfbCalls,
			},
			FinishReason: finishReason,
		},
		},
		Usage: Usage{
			PromptTokens:     res.Metadata.InputTokens,
			CompletionTokens: res.Metadata.OutputTokens,
			TotalTokens:      res.Metadata.TotalTokens,
		},
	}

	resp := BenchmarkResponse{
		Completion:     completion,
		ToolmanHistory: toolmanConversation,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// getToolCalls extracts prompts from response
func (i *Instance) getToolCalls(res *gen.Response) ([]prompt.Prompt, []ToolCall, error) {
	// response is assistant text
	if !res.IsTools() { // --> res.IsText()
		text, err := res.AsText()
		if err != nil {
			return nil, nil, err
		}
		assistant := prompt.AsAssistant(text)
		return []prompt.Prompt{assistant}, nil, nil
	}

	// response is tool calls
	var toolmanCalls []prompt.Prompt
	var cfbCalls []ToolCall
	for _, tool := range res.Tools {
		// PTC Tool Call
		if tool.Name == ptc.ToolName {
			// Unmarshal the 'argument' string/bytes to get the JS code
			var codeArgs struct {
				Code string `json:"code"`
			}
			err := json.Unmarshal(tool.Argument, &codeArgs)
			if err != nil {
				return nil, nil, err
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
		call, err := toolmanToCFBCall(tool)
		if err != nil {
			log.Fatalf("error: %e", err)
		}
		cfbCalls = append(cfbCalls, call)
	}

	return toolmanCalls, cfbCalls, nil
}

// executionReplay runs execution replay and returns bench response or tool response
func (i *Instance) executionReplay(bellmanTools []tools.Tool, toolmanConversation []prompt.Prompt, genResponse *gen.Response, model gen.Model) (*BenchmarkResponse, *prompt.Prompt) {
	result := i.Replay.ExecutionReplay(bellmanTools)
	if result.Error != nil {
		log.Fatalf("error: %e", result.Error)
	}

	// record --> bench tool call
	if result.Record != nil {
		call, err := recordToCFBCall(result.Record)
		if err != nil {
			log.Fatalf("error: %e", err)
		}

		// trace code execution
		jsonBytes, err := json.Marshal(result.Record.Argument)
		if err != nil {
			log.Printf("error: Error marshaling arguments: %v\n", err)
		}
		toolCall := prompt.AsToolCall(result.ToolID, result.Record.ToolName, jsonBytes)
		i.Tracer.TraceExec(toolCall)

		inputTokens := 0
		outputTokens := 0
		totalTokens := 0
		// set token count if llm response was generated
		if genResponse != nil {
			inputTokens = genResponse.Metadata.InputTokens
			outputTokens = genResponse.Metadata.OutputTokens
			totalTokens = genResponse.Metadata.TotalTokens
		}

		// return call, only 1 at a time
		finishReason := "tool_calls"

		completion := ChatCompletionResponse{
			ID:      "chatcmpl-123", // Important: fill with mock data! (for completion parsing in cfb)
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   model.String(),
			Choices: []Choice{{
				Index: 0,
				Message: ResponseMessage{
					Role:      "assistant",
					Content:   "",
					ToolCalls: []ToolCall{call},
				},
				FinishReason: finishReason,
			},
			},
			Usage: Usage{
				PromptTokens:     inputTokens,
				CompletionTokens: outputTokens,
				TotalTokens:      totalTokens,
			},
		}

		resp := BenchmarkResponse{
			Completion:     completion,
			ToolmanHistory: toolmanConversation,
		}

		return &resp, nil
	}

	// execution result --> toolman response
	toolResponse := prompt.AsToolResponse(result.ToolID, ptc.ToolName, result.Output)
	return nil, &toolResponse
}

// recordToCFBCall converts replay record to cfb tool call
func recordToCFBCall(record *replay.CallRecord) (ToolCall, error) {
	jsonBytes, err := json.Marshal(record.Argument)
	if err != nil {
		log.Printf("Error marshaling arguments: %v\n", err)
		return ToolCall{}, err
	}

	call := ToolCall{
		Type: "function",
		Function: ToolCallFunction{
			Name:      record.ToolName,
			Arguments: string(jsonBytes),
		},
	}
	return call, nil
}

// toolmanToCFBCall converts toolman call to cfb tool call
func toolmanToCFBCall(tool tools.Call) (ToolCall, error) {
	call := ToolCall{
		ID:   tool.ID,
		Type: "function",
		Function: ToolCallFunction{
			Name:      tool.Name,
			Arguments: string(tool.Argument),
		},
	}
	return call, nil
}

// ensureCache clears cache on new test (only user messages inbound)
func (c *Cache) ensureCache(req BenchmarkRequest) *Instance {
	c.mu.Lock()

	ptcFlag := "regular-fc" // regular function calling
	if req.EnablePTC {
		ptcFlag = "ptc-fc"
	}

	i, ok := c.Instances[req.TestID]
	if !ok {
		i = &Instance{
			Replay: replay.NewReplay(),
			Tracer: tracer.NewTracer(fmt.Sprintf("%s-%s-%s", req.TestID, ptcFlag, req.Model)),
		}
		i.timer = time.AfterFunc(3*time.Minute, func() {
			c.finish(req.TestID)
		})
		c.Instances[req.TestID] = i
	} else {
		i.timer.Reset(3 * time.Minute)
	}
	c.mu.Unlock()

	i.mu.Lock()
	defer i.mu.Unlock()

	// reset cache if only user message, and no new responses
	reset := true
	for _, m := range req.Messages {
		if m.Role != "user" {
			reset = false
			break
		}
	}
	if reset && (len(req.NewToolResponses) > 0 || len(req.ToolmanHistory) > 1) {
		reset = false
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
	// add trailing messages from CFB
	cfbUserCount := 0
	for _, m := range req.Messages {
		switch m.Role {
		case "user":
			// only add new user messages from cfb (not in toolman hist.)
			cfbUserCount++
			if cfbUserCount > toolmanUserCount {
				// update turn index & trace
				i.Tracer.NewTurn()
				userPrompt := prompt.AsUser(m.Content)
				i.Tracer.Trace(userPrompt, toolmanHistory, nil)
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
				i.Tracer.Trace(*response, nil, nil)
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
						i.Tracer.Trace(responsePrompt, nil, nil)
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
