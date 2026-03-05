package bfcl

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/modfin/bellman"
	"github.com/modfin/bellman/models/gen"
	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/tools"
	"github.com/modfin/bellman/tools/ptc"
	"github.com/modfin/bellman/tools/ptc/bench/replay"
	"github.com/modfin/bellman/tools/ptc/bench/utils"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.39.0"
	"go.opentelemetry.io/otel/trace"
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
	*replay.Replay
	Tracer *Tracer
}

type Tracer struct {
	Provider  *sdktrace.TracerProvider
	Tracer    trace.Tracer
	TestSpan  Span
	TurnSpan  Span
	ChatSpan  trace.Span
	ToolSpans map[string]Span
	ExecSpan  trace.Span
	Turn      int
}

type Span struct {
	trace.Span
	Context context.Context
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
				c.AddResponse(replay.CallRecord{
					ToolName: m.ToolName,
					Result:   m.Content,
				})
				// trace code execution
				toolResponse := prompt.AsToolResponse(m.ToolID, m.ToolName, m.Content)
				c.traceExec(toolResponse)
			}
		}
		// while there are scripts to run, replay them
		for c.IsPending() {
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

	// remove bfcl system prompt for PTC - misleading!
	if req.EnablePTC {
		req.SystemPrompt = ""
	}

	// trace llm call start (if not recording already)
	if c.Tracer.ChatSpan == nil || !c.Tracer.ChatSpan.IsRecording() {
		c.trace(prompt.AsUser("..."), toolmanConversation, model)
	}

	llm := client.Generator().Model(model).
		System(req.SystemPrompt).
		SetTools(bellmanTools...).
		SetPTCLanguage(tools.JavaScript) //.Temperature(req.Temperature)

	res, err := llm.Prompt(toolmanConversation...)
	if err != nil {
		log.Printf("Prompt Error: %v", err)

		// Catch the error, record it in otel, and cleanly close the trace!
		if c.Tracer.ChatSpan != nil && c.Tracer.ChatSpan.IsRecording() {
			// This adds a red error badge to the span in Langfuse
			c.Tracer.ChatSpan.RecordError(err)
		}
		// Force close the entire test trace so it exports properly
		c.sendTrace(true)

		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// log token usage
	logExecution(res)

	// get tool call or text response, and add PTC scripts to cache
	toolmanCalls, bfclCalls, bfclToolIDs, err := c.getToolCalls(res, toolmanConversation)
	if err != nil {
		log.Printf("error getting prompts: %v", err)

		// Catch the error, record it in otel, and cleanly close the trace!
		if c.Tracer.ChatSpan != nil && c.Tracer.ChatSpan.IsRecording() {
			// This adds a red error badge to the span in Langfuse
			c.Tracer.ChatSpan.RecordError(err)
		}
		// Force close the entire test trace so it exports properly
		c.sendTrace(true)

		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	toolmanConversation = append(toolmanConversation, toolmanCalls...)

	// trace tool calls
	for _, call := range toolmanCalls {
		c.trace(call, nil, model)
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
func (c *Cache) getToolCalls(res *gen.Response, history []prompt.Prompt) ([]prompt.Prompt, []ExtractedCall, []string, error) {
	var bfclCalls []ExtractedCall
	var bfclToolIDs []string

	// response is assistant text
	if !res.IsTools() { // --> res.IsText()
		text, err := res.AsText()
		if err != nil {
			return nil, nil, nil, err
		}
		model, err := gen.ToModel(res.Metadata.Model)
		if err != nil {
			log.Fatalf("error: %e", err)
		}
		assistant := prompt.AsAssistant(text)
		// trace new assistant prompt
		c.trace(assistant, history, model)
		return []prompt.Prompt{assistant}, nil, nil, nil
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
			c.AddScript(replay.Script{
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
	result := c.ExecutionReplay(bellmanTools)
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
		c.traceExec(toolCall)

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
		c.Clear()
		c.newTrace(req.TestID, req.SystemPrompt)
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
				model, err := gen.ToModel(req.Model)
				if err != nil {
					log.Fatalf("error: %e", err)
				}
				// update turn index & trace
				c.newTurn()
				userPrompt := prompt.AsUser(m.Content)
				c.trace(userPrompt, toolmanHistory, model)
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
				model, err := gen.ToModel(req.Model)
				if err != nil {
					log.Fatalf("error: %e", err)
				}
				// trace tool response
				c.trace(*response, nil, model)
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
						model, err := gen.ToModel(req.Model)
						if err != nil {
							log.Fatalf("error: %e", err)
						}
						// trace tool response
						responsePrompt := prompt.AsToolResponse(m.ToolID, m.ToolName, m.Content)
						c.trace(responsePrompt, nil, model)
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

// trace automatically traces prompts
func (c *Cache) trace(p prompt.Prompt, history []prompt.Prompt, model gen.Model) {
	// add spans to trace
	chatSpan := c.Tracer.ChatSpan
	var toolSpan Span

	switch p.Role {
	case prompt.UserRole:
		if chatSpan == nil || !chatSpan.IsRecording() {
			_, chatSpan = c.Tracer.Tracer.Start(c.Tracer.TurnSpan.Context, fmt.Sprintf("chat %s", model.Name))
			// test input message history
			jsonData, err := json.Marshal(history)
			if err != nil {
				log.Printf("Failed to marshal gen_ai.input.messages: %v", err)
				return
			}
			chatSpan.SetAttributes(
				attribute.String("gen_ai.operation.name", "chat"),
				attribute.String("gen_ai.provider.name", model.Provider),
				attribute.String("gen_ai.request.model", model.Name),
				attribute.String("gen_ai.input.messages", string(jsonData)),
				attribute.String("gen_ai.prompt", p.Text),
			)
		}
	case prompt.AssistantRole:
		if chatSpan == nil || !chatSpan.IsRecording() {
			_, chatSpan = c.Tracer.Tracer.Start(c.Tracer.TurnSpan.Context, fmt.Sprintf("chat %s", model.Name))
			// test input message history
			jsonData, err := json.Marshal(history)
			if err != nil {
				log.Printf("Failed to marshal gen_ai.input.messages: %v", err)
				return
			}
			chatSpan.SetAttributes(
				attribute.String("gen_ai.operation.name", "chat"),
				attribute.String("gen_ai.provider.name", model.Provider),
				attribute.String("gen_ai.request.model", model.Name),
				attribute.String("gen_ai.input.messages", string(jsonData)),
				attribute.String("gen_ai.prompt", fmt.Sprintf("Conversation history...")),
			)
		}
		chatSpan.SetAttributes(
			attribute.String("gen_ai.completion", p.Text),
		)
		chatSpan.End()
		time.Sleep(1 * time.Millisecond) // sleep 1ms to enforce otel order
	case prompt.ToolCallRole:
		if chatSpan != nil {
			chatSpan.SetAttributes(
				attribute.String("gen_ai.completion", fmt.Sprintf("Tool Call Requested: %v", p.ToolCall.Name)),
			)
			chatSpan.End()
			time.Sleep(1 * time.Millisecond) // sleep 1ms to enforce otel order
		}
		// Immediately open a Tool Span!
		//toolSpan = c.Tracer.ToolSpans[p.ToolResponse.ToolCallID]
		toolSpan.Context, toolSpan.Span = c.Tracer.Tracer.Start(c.Tracer.TurnSpan.Context, fmt.Sprintf("execute_tool %s", p.ToolCall.Name))
		toolSpan.SetAttributes(
			attribute.String("gen_ai.operation.name", "execute_tool"),
			attribute.String("gen_ai.tool.name", p.ToolCall.Name),
			attribute.String("gen_ai.tool.call.arguments", string(p.ToolCall.Arguments)),
			attribute.String("gen_ai.tool.call.id", p.ToolCall.ToolCallID),
		)
		c.Tracer.ToolSpans[p.ToolCall.ToolCallID] = toolSpan
	case prompt.ToolResponseRole:
		toolSpan = c.Tracer.ToolSpans[p.ToolResponse.ToolCallID]
		if toolSpan.Span != nil {
			// The tool finished executing! Log the result and close the chatSpan.
			toolSpan.SetAttributes(
				attribute.String("gen_ai.tool.call.result", p.ToolResponse.Response),
			)
			toolSpan.End()
			time.Sleep(1 * time.Millisecond) // sleep 1ms to enforce otel order
		}
		c.Tracer.ToolSpans[p.ToolResponse.ToolCallID] = toolSpan
	}
	// Save the state back to the struct
	c.Tracer.ChatSpan = chatSpan
}

func (c *Cache) traceExec(p prompt.Prompt) {
	// add spans to trace
	execSpan := c.Tracer.ExecSpan

	switch p.Role {
	case prompt.ToolCallRole:
		// Immediately open a Tool Span!
		_, execSpan = c.Tracer.Tracer.Start(c.Tracer.ToolSpans[p.ToolCall.ToolCallID].Context, fmt.Sprintf("execute_tool %s", p.ToolCall.Name))
		execSpan.SetAttributes(
			attribute.String("gen_ai.operation.name", "execute_tool"),
			attribute.String("gen_ai.tool.name", p.ToolCall.Name),
			attribute.String("gen_ai.tool.call.arguments", string(p.ToolCall.Arguments)),
			attribute.String("gen_ai.tool.call.id", p.ToolCall.ToolCallID),
		)
	case prompt.ToolResponseRole:
		if execSpan != nil {
			// The tool finished executing! Log the result and close the chatSpan.
			execSpan.SetAttributes(
				attribute.String("gen_ai.tool.call.result", p.ToolResponse.Response),
			)
			execSpan.End()
			time.Sleep(1 * time.Millisecond) // sleep 1ms to enforce otel order
		}
	}
	// Save the state back to the struct
	c.Tracer.ExecSpan = execSpan
}

// newTrace setup new trace
func (c *Cache) newTrace(testID string, system string) {
	// init tracer
	c.setupHttpLangfuse()

	// if a previous benchmark was running, close its spans to send telemetry
	c.sendTrace(true)

	ctx := context.Background()

	// reset turn index
	c.Tracer.Turn = 0

	// empty tool spans map
	c.Tracer.ToolSpans = make(map[string]Span)

	// Create the PARENT Span. This represents the entire conversational session.
	// By reassigning to 'ctx', all future spans will become children of this trace.
	c.Tracer.TestSpan.Context, c.Tracer.TestSpan.Span = c.Tracer.Tracer.Start(ctx, fmt.Sprintf("%s", testID))

	c.Tracer.TestSpan.SetAttributes(
		attribute.String("gen_ai.system_instructions", system),
	)
}

func (c *Cache) newTurn() {
	c.sendTrace(false)
	c.Tracer.TurnSpan.Context, c.Tracer.TurnSpan.Span = c.Tracer.Tracer.Start(c.Tracer.TestSpan.Context, fmt.Sprintf("turn_%d", c.Tracer.Turn))
	c.Tracer.Turn++
}

func (c *Cache) sendTrace(sendTest bool) {
	if c.Tracer.ExecSpan != nil && c.Tracer.ExecSpan.IsRecording() {
		c.Tracer.ExecSpan.End()
	}
	for _, s := range c.Tracer.ToolSpans {
		if s.Span != nil && s.IsRecording() {
			s.End()
		}
	}
	if c.Tracer.ChatSpan != nil && c.Tracer.ChatSpan.IsRecording() {
		c.Tracer.ChatSpan.End()
	}
	if c.Tracer.TurnSpan.Span != nil && c.Tracer.TurnSpan.IsRecording() {
		c.Tracer.TurnSpan.End()
	}
	if sendTest && c.Tracer.TestSpan.Span != nil && c.Tracer.TestSpan.IsRecording() {
		c.Tracer.TestSpan.End()
	}
}

// setupHttpLangfuse reads the .env and wires a direct HTTP connection
func (c *Cache) setupHttpLangfuse() {
	// only create new on startup
	if c.Tracer == nil {
		c.Tracer = &Tracer{}
	}
	if c.Tracer.Tracer != nil {
		return
	}
	if c.Tracer.ToolSpans == nil {
		c.Tracer.ToolSpans = make(map[string]Span)
	}

	ctx := context.Background()

	// Load the keys
	_ = godotenv.Load()
	pubKey := os.Getenv("LANGFUSE_PUBLIC_KEY")
	secKey := os.Getenv("LANGFUSE_SECRET_KEY")
	host := os.Getenv("LANGFUSE_BASE_URL")
	if pubKey == "" || secKey == "" {
		log.Fatal("Missing LANGFUSE_PUBLIC_KEY or LANGFUSE_SECRET_KEY in .env")
	}

	// Base64 encode for Basic Auth
	auth := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", pubKey, secKey)))

	// Configure the HTTP Exporter directly to your local Docker container
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(host),
		otlptracehttp.WithURLPath("/api/public/otel/v1/traces"),
		otlptracehttp.WithInsecure(), // REQUIRED for localhost testing without HTTPS!
		otlptracehttp.WithHeaders(map[string]string{
			"Authorization": "Basic " + auth,
		}),
	)
	if err != nil {
		log.Fatalf("Failed to create HTTP exporter: %v", err)
	}

	// Create and set the Tracer Provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("BFCL"),
		)),
	)
	otel.SetTracerProvider(tp)
	c.Tracer.Provider = tp
	c.Tracer.Tracer = otel.Tracer("benchmark")

	// set channel listener to send traces on exit
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan // blocks until Ctrl+C or kill process

		fmt.Println("\n[Telemetry] Shutting down... Flushing traces to Langfuse!")

		// force close any spans currently active in memory
		c.sendTrace(true)

		// force provider to flush pending HTTP requests
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := c.Tracer.Provider.Shutdown(shutdownCtx); err != nil {
			log.Printf("Error flushing telemetry: %v", err)
		}

		os.Exit(0) // now exit safely
	}()
}
