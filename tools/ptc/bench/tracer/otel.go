package tracer

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/modfin/bellman/models/gen"
	"github.com/modfin/bellman/prompt"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.39.0"
	"go.opentelemetry.io/otel/trace"
)

type Tracer struct {
	Provider   *sdktrace.TracerProvider
	Tracer     trace.Tracer
	RootSpan   Span
	TurnSpan   Span
	ChatSpan   Span
	ToolSpans  map[string]Span
	ExecSpan   Span
	Turn       int
	ToolString string
	Model      gen.Model
}

type Span struct {
	trace.Span
	Context context.Context
}

type TracerRequest struct {
	Model          string          `json:"model"`
	ToolmanHistory []prompt.Prompt `json:"toolman_history"`
	Tools          []interface{}   `json:"tools"`
	SystemPrompt   string          `json:"system_prompt"`
	TestID         string          `json:"test_id"`
}

// NewTracer creates a new cache
func NewTracer(name string) *Tracer {
	t := &Tracer{}
	t.SetupHttpLangfuse(name)
	return t
}

// Trace automatically traces prompts
func (t *Tracer) Trace(p prompt.Prompt, messages []prompt.Prompt) {
	// add spans to trace
	chatSpan := t.ChatSpan
	var toolSpan Span

	switch p.Role {
	case prompt.UserRole:
		if chatSpan.Span == nil || !chatSpan.IsRecording() {
			chatSpan.Context, chatSpan.Span = t.Tracer.Start(t.TurnSpan.Context, fmt.Sprintf("chat %s", t.Model.Name))
			// add input message conversation
			jsonConversation, _ := json.MarshalIndent(messages, "", "  ")
			chatSpan.SetAttributes(
				attribute.String("gen_ai.operation.name", "chat"),
				attribute.String("gen_ai.provider.name", t.Model.Provider),
				attribute.String("gen_ai.request.model", t.Model.Name),
				attribute.String("gen_ai.input.messages", string(jsonConversation)),
				attribute.String("gen_ai.prompt", p.Text),
				attribute.String("gen_ai.tool.definitions", t.ToolString),
			)
		}
	case prompt.AssistantRole:
		if chatSpan.Span == nil || !chatSpan.IsRecording() {
			chatSpan.Context, chatSpan.Span = t.Tracer.Start(t.TurnSpan.Context, fmt.Sprintf("chat %s", t.Model.Name))
			// add input message conversation
			//jsonResponse, _ := json.MarshalIndent(messages, "", "  ")
			chatSpan.SetAttributes(
				attribute.String("gen_ai.operation.name", "chat"),
				attribute.String("gen_ai.provider.name", t.Model.Provider),
				attribute.String("gen_ai.response.model", t.Model.Name),
				//attribute.String("gen_ai.output.messages", string(jsonResponse)),
				attribute.String("gen_ai.prompt", fmt.Sprintf("Conversation history...")),
			)
		}
		chatSpan.SetAttributes(
			attribute.String("gen_ai.completion", p.Text),
		)
		chatSpan.End()
		time.Sleep(1 * time.Millisecond) // sleep 1ms to enforce otel order
	case prompt.ToolCallRole:
		if chatSpan.Span != nil {
			// add input message conversation
			jsonResponse, _ := json.MarshalIndent(messages, "", "  ")
			chatSpan.SetAttributes(
				attribute.String("gen_ai.output.messages", string(jsonResponse)),
				attribute.String("gen_ai.completion", "Tool Calls Requested"),
			)
			chatSpan.End()
			time.Sleep(1 * time.Millisecond) // sleep 1ms to enforce otel order
		}
		// Immediately open a Tool Span!
		//toolSpan = t.ToolSpans[p.ToolResponse.ToolCallID]
		toolSpan.Context, toolSpan.Span = t.Tracer.Start(t.TurnSpan.Context, fmt.Sprintf("execute_tool %s", p.ToolCall.Name))
		toolSpan.SetAttributes(
			attribute.String("gen_ai.operation.name", "execute_tool"),
			attribute.String("gen_ai.tool.name", p.ToolCall.Name),
			attribute.String("gen_ai.tool.call.arguments", string(p.ToolCall.Arguments)),
			attribute.String("gen_ai.tool.call.id", p.ToolCall.ToolCallID),
		)
		t.ToolSpans[p.ToolCall.ToolCallID] = toolSpan
	case prompt.ToolResponseRole:
		toolSpan = t.ToolSpans[p.ToolResponse.ToolCallID]
		if toolSpan.Span != nil {
			// The tool finished executing! Log the result and close the chatSpan.
			toolSpan.SetAttributes(
				attribute.String("gen_ai.tool.call.result", p.ToolResponse.Response),
			)
			toolSpan.End()
			time.Sleep(1 * time.Millisecond) // sleep 1ms to enforce otel order
		}
		t.ToolSpans[p.ToolResponse.ToolCallID] = toolSpan
	}
	// Save the state back to the struct
	t.ChatSpan = chatSpan
}

// TraceExec traces code execution function calls
func (t *Tracer) TraceExec(p prompt.Prompt) {
	// add spans to trace
	execSpan := t.ExecSpan

	switch p.Role {
	case prompt.ToolCallRole:
		// Immediately open a Tool Span!
		execSpan.Context, execSpan.Span = t.Tracer.Start(t.ToolSpans[p.ToolCall.ToolCallID].Context, fmt.Sprintf("execute_tool %s", p.ToolCall.Name))
		execSpan.SetAttributes(
			attribute.String("gen_ai.operation.name", "execute_tool"),
			attribute.String("gen_ai.tool.name", p.ToolCall.Name),
			attribute.String("gen_ai.tool.call.arguments", string(p.ToolCall.Arguments)),
			attribute.String("gen_ai.tool.call.id", p.ToolCall.ToolCallID),
		)
	case prompt.ToolResponseRole:
		if execSpan.Span != nil {
			// The tool finished executing! Log the result and close the chatSpan.
			execSpan.SetAttributes(
				attribute.String("gen_ai.tool.call.result", p.ToolResponse.Response),
			)
			execSpan.End()
			time.Sleep(1 * time.Millisecond) // sleep 1ms to enforce otel order
		}
	}
	// Save the state back to the struct
	t.ExecSpan = execSpan
}

// TraceError traces an error on a span and sends all spans
func (t *Tracer) TraceError(span Span, err error) {
	// Catch the error, record it in otel, and cleanly close the trace!
	if span.Span != nil && span.IsRecording() {
		// This adds a red error badge to the span in Langfuse
		span.RecordError(err)
	}
	// Force close the entire test trace so it exports properly
	t.SendTrace(true)
}

// NewTrace setup new trace
func (t *Tracer) NewTrace(req TracerRequest) {
	// if a previous benchmark was running, close its spans to send telemetry
	t.SendTrace(true)

	ctx := context.Background()

	// add tool input
	toolsBytes, _ := json.MarshalIndent(req.Tools, "", "  ")
	t.ToolString = string(toolsBytes)

	// add model
	model, err := gen.ToModel(req.Model)
	if err == nil {
		t.Model = model
	}

	// reset turn index
	t.Turn = 0

	// empty tool spans map
	t.ToolSpans = make(map[string]Span)

	// Create the PARENT Span. This represents the entire conversational session.
	// By reassigning to 'ctx', all future spans will become children of this trace.
	t.RootSpan.Context, t.RootSpan.Span = t.Tracer.Start(ctx, fmt.Sprintf("%s", req.TestID))

	t.RootSpan.SetAttributes(
		attribute.String("gen_ai.system_instructions", req.SystemPrompt),
	)
}

// NewTurn starts a new turn span
func (t *Tracer) NewTurn() {
	t.SendTrace(false)
	t.TurnSpan.Context, t.TurnSpan.Span = t.Tracer.Start(t.RootSpan.Context, fmt.Sprintf("turn_%d", t.Turn))
	t.Turn++
}

// SendTrace sends all trace spans
func (t *Tracer) SendTrace(sendTest bool) {
	if t.ExecSpan.Span != nil && t.ExecSpan.IsRecording() {
		t.ExecSpan.End()
	}
	for _, s := range t.ToolSpans {
		if s.Span != nil && s.IsRecording() {
			s.End()
		}
	}
	if t.ChatSpan.Span != nil && t.ChatSpan.IsRecording() {
		t.ChatSpan.End()
	}
	if t.TurnSpan.Span != nil && t.TurnSpan.IsRecording() {
		t.TurnSpan.End()
	}
	if sendTest && t.RootSpan.Span != nil && t.RootSpan.IsRecording() {
		t.RootSpan.End()
	}
}

// SetupHttpLangfuse reads the .env and wires a direct HTTP connection
func (t *Tracer) SetupHttpLangfuse(name string) {
	ctx := context.Background()
	t.ToolSpans = make(map[string]Span)

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
			semconv.ServiceName("toolman"),
		)),
	)
	otel.SetTracerProvider(tp)
	t.Provider = tp
	t.Tracer = otel.Tracer(name)

	// set channel listener to send traces on exit
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan // blocks until Ctrl+C or kill process

		fmt.Println("\n[Telemetry] Shutting down... Flushing traces to Langfuse!")

		// force close any spans currently active in memory
		t.SendTrace(true)

		// force provider to flush pending HTTP requests
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := t.Provider.Shutdown(shutdownCtx); err != nil {
			log.Printf("Error flushing telemetry: %v", err)
		}

		os.Exit(0) // now exit safely
	}()
}
