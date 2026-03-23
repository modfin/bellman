package nestful

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/dop251/goja"
	"github.com/joho/godotenv"
	"github.com/modfin/bellman"
	"github.com/modfin/bellman/models/gen"
	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/schema"
	"github.com/modfin/bellman/services/openai"
	"github.com/modfin/bellman/tools"
	"github.com/modfin/bellman/tools/ptc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

// --- NESTFUL benchmark adapter (single-shot, with/without PTC) ---

type NestfulBenchmarkRequest struct {
	Model              string  `json:"bellman_model"` // provider/name
	Query              string  `json:"query"`
	Tools              []any   `json:"tools"`
	Temperature        float64 `json:"temperature"`
	MaxTokens          int     `json:"max_tokens"`
	BatchSize          int     `json:"batch_size"`
	SystemPrompt       string  `json:"system_prompt"`
	EnablePTC          bool    `json:"enable_ptc"`
	ToolChoice         string  `json:"tool_choice,omitempty"` // auto|required|none
	JSExtractTimeoutMs int     `json:"js_extract_timeout_ms,omitempty"`
}

type NestfulBenchmarkResponse struct {
	GeneratedText string `json:"generated_text"` // JSON list string, NESTFUL scorer input
	Content       string `json:"content,omitempty"`
	InputTokens   int    `json:"input_tokens"`
	OutputTokens  int    `json:"output_tokens"`
	TotalTokens   int    `json:"total_tokens"`
}

type nestfulToolDef struct {
	Name             string         `json:"name"`
	Description      string         `json:"description"`
	Parameters       map[string]any `json:"parameters"`
	OutputParameters map[string]any `json:"output_parameters"`
}

type extractMeta struct {
	GuardrailOK       bool
	CapturedCount     int
	CapturedJSONTrunc string
}

// Regex to find invalid tool-name characters.
var invalidNameChars = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

func NesfulHandlerFromEnv() http.HandlerFunc {
	_ = godotenv.Load(".env")
	bellmanURL := os.Getenv("BELLMAN_URL")
	bellmanToken := os.Getenv("BELLMAN_TOKEN")

	client := bellman.New(bellmanURL, bellman.Key{Name: "nestful", Token: bellmanToken})
	model := openai.GenModel_gpt5_mini_250807
	//model := vertexai.GenModel_gemini_2_5_flash_latest

	ctx := context.Background()
	tp, err := setupHttpLangfuse(ctx)

	if err != nil {
		fmt.Println("otel desabled: ", err)
	} else {
		_ = tp
	}

	return NestfulHandlerWrapper(client, model)
}

// NestfulHandler exposes a single-shot endpoint that returns predicted tool-call sequences in NESTFUL's format.
//
// - If EnablePTC=false: reads res.Tools and returns [{name,arguments,label}, ...]
// - If EnablePTC=true: expects code_execution; runs JS in goja with interceptors and returns the extracted sequence.
//
// Tools are never executed.

func NestfulHandler(w http.ResponseWriter, r *http.Request, client *bellman.Bellman, model gen.Model) {

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req NestfulBenchmarkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpErr(w, fmt.Errorf("invalid json: %w", err), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Query) == "" {
		httpErr(w, fmt.Errorf("query is required"), http.StatusBadRequest)
		return
	}
	if req.MaxTokens <= 0 {
		req.MaxTokens = 1000
	}
	if req.JSExtractTimeoutMs <= 0 {
		req.JSExtractTimeoutMs = 5000
	}
	choice := strings.ToLower(strings.TrimSpace(req.ToolChoice))
	if choice == "" {
		choice = "required"
	}
	tracer := otel.Tracer("toolman/nestful")
	ctx := r.Context()

	sampleID := benchSampleID(r)
	ctx, root := tracer.Start(ctx, "nestful.request")
	defer root.End()

	root.SetAttributes(
		attribute.String("benchmark.name", "nestful"),
		attribute.String("benchmark.sample_id", sampleID),
		attribute.Bool("ptc.enabled", req.EnablePTC),
		attribute.String("tool.choice", choice),
	)

	root.SetAttributes(
		attribute.String("gen_ai.request.model", fmt.Sprintf("%v/%v", model.Provider, model.Name)),
	)

	parsedTools, nameMap, outKeysByTool, err := parseNestfulTools(req.Tools)
	if err != nil {
		root.RecordError(err)
		root.SetStatus(codes.Error, err.Error())
		httpErr(w, fmt.Errorf("invalid tools: %w", err), http.StatusBadRequest)
		return
	}

	root.SetAttributes(
		attribute.Int("gen_ai.tools.count", len(parsedTools)),
		attribute.String("gen_ai.tools.digest", toolsDigest(req.Tools)),
	)

	if len(parsedTools) <= 50 {
		names := make([]string, 0, len(parsedTools))
		for _, t := range parsedTools {
			names = append(names, t.Name)
		}
		root.SetAttributes(attribute.String("gen_ai.tools.names", strings.Join(names, ",")))
	}
	var callIdx uint64
	for i := range parsedTools {
		parsedTools[i].UsePTC = req.EnablePTC
		// Never executed; just to keep tool refs non-nil.
		//parsedTools[i].Function = func(ctx context.Context, call tools.Call) (string, error) { return "{}", nil }
		parsedTools[i].Function = func(ctx context.Context, call tools.Call) (string, error) {
			idx := atomic.AddUint64(&callIdx, 1)

			// call.Name is the sanitized tool name (matches outKeysByTool keys).
			keys := outKeysByTool[call.Name]
			if len(keys) == 0 {
				keys = []string{"result"}
			}

			out := make(map[string]any, len(keys))
			for _, k := range keys {
				out[k] = fmt.Sprintf("$var_%d.%s$", idx, k)
			}

			b, _ := json.Marshal(out)
			return string(b), nil

		}

	}
	llm := client.Generator().
		Model(model).
		System(req.SystemPrompt).
		SetTools(parsedTools...)
	//Temperature(req.Temperature).
	//MaxTokens(req.MaxTokens)

	if req.EnablePTC {
		llm, err = llm.ActivatePTC(ptc.JavaScript)
		if err != nil {
			log.Printf("failed to activate ptc: %v", err)
		}
	}

	var res *gen.Response

	llmCtx, llmSpan := tracer.Start(ctx, "llm.prompt")
	defer llmSpan.End()

	llmSpan.SetAttributes(
		attribute.String("gen_ai.operation.name", "chat"),
		attribute.String("gen_ai.request.model", fmt.Sprintf("%v/%v", model.Provider, model.Name)),
		attribute.String("gen_ai.system_instructions", truncate(req.SystemPrompt, 4000)),
		attribute.String("gen_ai.prompt", truncate(req.Query, 8000)),
	)

	res, err = llm.Prompt(prompt.AsUser(req.Query))
	//fmt.Println("LMM resp", res.Tools)

	if err != nil {
		llmSpan.RecordError(err)
		llmSpan.SetStatus(codes.Error, err.Error())
		//llmSpan.End()
		root.RecordError(err)
		root.SetStatus(codes.Error, "llm prompt failed")

		writeJSON(w, http.StatusOK, NestfulBenchmarkResponse{
			GeneratedText: "[]",
			Content:       fmt.Sprintf("llm prompt error: %v", err),
			InputTokens:   0,
			OutputTokens:  0,
			TotalTokens:   0,
		})
		return

	} else {
		llmSpan.SetAttributes(
			attribute.Int("gen_ai.usage.input_tokens", res.Metadata.InputTokens),
			attribute.Int("gen_ai.usage.output_tokens", res.Metadata.OutputTokens),
			attribute.Int("gen_ai.usage.total_tokens", res.Metadata.TotalTokens),
		)
	}

	/*if err != nil {
		httpErr(w, fmt.Errorf("upstream error: %w", err), http.StatusBadGateway)
		return
	}*/

	//tracer := otel.Tracer("toolman/nestful")
	generated, content := nestfulGeneratedText(llmCtx, tracer, res, parsedTools, nameMap, outKeysByTool, req.JSExtractTimeoutMs)
	if strings.TrimSpace(generated) == "" {
		generated = "[]"
	}
	//llmSpan.End()
	writeJSON(w, http.StatusOK, NestfulBenchmarkResponse{
		GeneratedText: generated,
		Content:       content,
		InputTokens:   res.Metadata.InputTokens,
		OutputTokens:  res.Metadata.OutputTokens,
		TotalTokens:   res.Metadata.TotalTokens,
	})
}

func NestfulHandlerWrapper(client *bellman.Bellman, model gen.Model) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		NestfulHandler(w, r, client, model)
	}
}

func parseNestfulTools(raw []any) ([]tools.Tool, map[string]string, map[string][]string, error) {
	// nameMap: sanitized -> original
	nameMap := map[string]string{}
	// outKeysByTool: sanitized tool name -> sorted output keys
	outKeysByTool := map[string][]string{}
	parsed := make([]tools.Tool, 0, len(raw))
	for _, rt := range raw {
		b, err := json.Marshal(rt)
		if err != nil {
			return nil, nil, nil, err
		}
		var def nestfulToolDef
		if err := json.Unmarshal(b, &def); err != nil {
			return nil, nil, nil, err
		}
		if strings.TrimSpace(def.Name) == "" {
			continue
		}
		orig := def.Name
		sanitized := invalidNameChars.ReplaceAllString(orig, "_")
		nameMap[sanitized] = orig

		outKeys := make([]string, 0, len(def.OutputParameters))
		for k := range def.OutputParameters {
			if strings.TrimSpace(k) == "" {
				continue
			}
			outKeys = append(outKeys, k)
		}
		if len(outKeys) == 0 {
			outKeys = []string{"result"}
		}
		sort.Strings(outKeys)
		outKeysByTool[sanitized] = outKeys

		s := &schema.JSON{Type: schema.Object, Properties: map[string]*schema.JSON{}}
		var required []string
		for k, v := range def.Parameters {
			ps := schemaFromAny(v)
			if ps == nil {
				ps = &schema.JSON{}
			}
			s.Properties[k] = ps
			if isRequired(v) {
				required = append(required, k)
			}
		}
		// NESTFUL tool specs often omit explicit per-parameter required flags.
		// Default to requiring *all* parameters when none are marked required.
		if len(required) == 0 {
			required = make([]string, 0, len(s.Properties))
			for k := range s.Properties {
				required = append(required, k)
			}
		}
		if len(required) > 0 {
			sort.Strings(required)
			s.Required = required
		}

		respSchema := &schema.JSON{
			Type:       schema.Object,
			Properties: map[string]*schema.JSON{},
		}

		for k, v := range def.OutputParameters {
			ps := schemaFromAny(v)
			if ps == nil {
				ps = &schema.JSON{}
			}
			respSchema.Properties[k] = ps
		}

		safe := strings.ReplaceAll(def.Description, "<", "`")
		safe = strings.ReplaceAll(safe, ">", "`")

		parsed = append(parsed, tools.Tool{
			Name:           sanitized,
			Description:    safe,
			ArgumentSchema: s,
			ResponseSchema: respSchema,
		})
	}
	return parsed, nameMap, outKeysByTool, nil
}

func nestfulGeneratedText(ctx context.Context, tracer trace.Tracer, res *gen.Response, availableTools []tools.Tool, nameMap map[string]string, outKeysByTool map[string][]string, timeoutMs int) (generated string, content string) {
	if !res.IsTools() {
		text, _ := res.AsText()
		return "[]", text
	}
	out := make([]map[string]any, 0)
	errMsgs := make([]string, 0, 1)
	for i, tc := range res.Tools {
		if tc.Name == "code_execution" {
			var codeArgs struct {
				Code string `json:"code"`
			}
			if err := json.Unmarshal(tc.Argument, &codeArgs); err != nil {
				errMsgs = append(errMsgs, fmt.Sprintf("code_execution args unmarshal error: %v", err))
				continue
			}
			seq, errMsg := executeAndExtractNestful(ctx, tracer, codeArgs.Code, availableTools, outKeysByTool, timeoutMs)
			if errMsg != "" {
				errMsgs = append(errMsgs, errMsg)
			}
			for i := range seq {
				if n, ok := seq[i]["name"].(string); ok {
					if orig, ok := nameMap[n]; ok {
						seq[i]["name"] = orig
					}
				}
			}
			out = append(out, seq...)
			continue
		}

		args := map[string]any{}
		_ = json.Unmarshal(tc.Argument, &args)
		_, toolSpan := tracer.Start(ctx, fmt.Sprintf("tool.call %s", tc.Name),
			trace.WithAttributes(
				attribute.String("gen_ai.tool.name", tc.Name),
				attribute.String("gen_ai.tool.call.arguments", string(mustJSON(args))),
				attribute.Int("index", i+1),
			),
		)
		toolSpan.End()
		name := tc.Name
		if orig, ok := nameMap[name]; ok {
			name = orig
		}
		out = append(out, map[string]any{"name": name, "arguments": args})
	}
	for i := range out {
		out[i]["label"] = fmt.Sprintf("$var_%d", i+1)
	}
	return string(mustJSON(out)), strings.Join(errMsgs, "\n")
}

func executeAndExtractNestful(
	ctx context.Context,
	tracer trace.Tracer,
	jsCode string,
	availableTools []tools.Tool,
	outKeysByTool map[string][]string,
	timeoutMs int,
) ([]map[string]any, string) {
	meta := extractMeta{GuardrailOK: false, CapturedCount: 0, CapturedJSONTrunc: ""}
	vm := goja.New()
	captured := make([]map[string]any, 0)

	runtime, err := ptc.NewRuntime(ptc.JavaScript)
	if err != nil {
		log.Fatalf("error: %e", err)
	}

	// Normalisera escaped radbrytningar/tabbar från modellen
	jsCode = strings.ReplaceAll(jsCode, "\\n", "\n")
	jsCode = strings.ReplaceAll(jsCode, "\\t", "\t")
	jsCode = strings.TrimSpace(jsCode)

	guarded, guardErr := runtime.Guardrail(jsCode)
	if guardErr != nil {
		return captured, fmt.Sprintf("code_execution guardrail error: %v", guardErr)
	}

	meta.GuardrailOK = true

	timer := time.AfterFunc(time.Duration(timeoutMs)*time.Millisecond, func() {
		vm.Interrupt("timeout")
	})
	defer timer.Stop()

	execCtx, execSpan := tracer.Start(ctx, "exec.goja")
	execSpan.SetAttributes(
		attribute.String("exec.language", "javascript"),
		attribute.Int("exec.script_len", len(guarded)),
		attribute.Int("exec.timeout_ms", timeoutMs),
		attribute.String("exec.script.trunc", truncate(guarded, 4000)),
	)
	defer execSpan.End()

	functionsObj := vm.NewObject()

	for _, t := range availableTools {
		tName := t.Name
		keys := outKeysByTool[tName]
		if len(keys) == 0 {
			keys = []string{"result"}
		}

		interceptor := func(tName string, keys []string) func(goja.FunctionCall) goja.Value {
			return func(call goja.FunctionCall) goja.Value {
				idx := len(captured) + 1

				outObj := make(map[string]any, len(keys))
				for _, k := range keys {
					outObj[k] = fmt.Sprintf("$var_%d.%s$", idx, k)
				}

				argsMap := make(map[string]any)
				if len(call.Arguments) > 0 {
					first := call.Arguments[0].Export()
					if obj, ok := first.(map[string]any); ok {
						for k, v := range obj {
							argsMap[k] = v
						}
					} else {
						argsMap["_error"] = "tool call used positional arguments; expected single object argument"
					}
				}

				normalized := normalizeVarRefs(argsMap)
				if m, ok := normalized.(map[string]any); ok {
					argsMap = m
				}

				captured = append(captured, map[string]any{
					"name":      tName,
					"arguments": argsMap,
				})

				_, toolSpan := tracer.Start(execCtx, fmt.Sprintf("tool.call %s", tName), trace.WithAttributes(
					attribute.String("gen_ai.operation.name", "execute_tool"),
					attribute.String("gen_ai.tool.name", tName),
					attribute.String("gen_ai.tool.call.arguments", string(mustJSON(argsMap))),
					attribute.Int("index", len(captured)),
				))
				toolSpan.End()

				return vm.ToValue(outObj)
			}
		}(tName, keys)

		if err := vm.Set(tName, interceptor); err != nil {
			return captured, fmt.Sprintf("code_execution binding error: %v", err)
		}
		if err := functionsObj.Set(tName, interceptor); err != nil {
			return captured, fmt.Sprintf("code_execution functions binding error: %v", err)
		}
	}

	if err := vm.Set("functions", functionsObj); err != nil {
		return captured, fmt.Sprintf("code_execution functions object error: %v", err)
	}

	if _, runErr := vm.RunString(guarded); runErr != nil {
		execSpan.RecordError(runErr)
		execSpan.SetStatus(codes.Error, runErr.Error())
		return captured, fmt.Sprintf("code_execution run error: %v", runErr)
	}

	execSpan.SetAttributes(
		attribute.String("output.value", "ok"),
		attribute.Int("captured.tool_calls", len(captured)),
		attribute.String("captured.json", string(mustJSON(captured))),
	)

	return captured, ""
}

// normalizeVarRefs converts nested {"result": "$var_i.result$"} values into the
// string "$var_i.result$" so arguments match NESTFUL's expected reference format.
func normalizeVarRefs(v any) any {
	switch x := v.(type) {
	case map[string]any:
		if len(x) == 1 {
			for _, vv := range x {
				if r, ok := vv.(string); ok && strings.HasPrefix(r, "$var_") && strings.HasSuffix(r, "$") {
					return r
				}
			}
		}
		for k, vv := range x {
			x[k] = normalizeVarRefs(vv)
		}
		return x
	case []any:
		for i := range x {
			x[i] = normalizeVarRefs(x[i])
		}
		return x
	default:
		return v
	}
}

func isRequired(pdef any) bool {
	m, ok := pdef.(map[string]any)
	if !ok {
		return false
	}
	v, ok := m["required"]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}

func schemaFromAny(v any) *schema.JSON {
	m, ok := v.(map[string]any)
	if !ok {
		if s, ok := v.(string); ok {
			js := &schema.JSON{}
			applyTypeFromString(js, s)
			return js
		}
		return nil
	}

	js := &schema.JSON{}
	if d, ok := m["description"].(string); ok {
		js.Description = d
	}
	if n, ok := m["nullable"].(bool); ok {
		js.Nullable = n
	}
	if typ, ok := m["type"]; ok {
		if ts, ok := typ.(string); ok {
			applyTypeFromString(js, ts)
		}
	}
	if js.Type == schema.Array {
		if items, ok := m["items"]; ok {
			js.Items = schemaFromAny(items)
			if js.Items == nil {
				js.Items = &schema.JSON{}
			}
		}
	}
	if js.Type == schema.Object {
		if props, ok := m["properties"].(map[string]any); ok {
			js.Properties = map[string]*schema.JSON{}
			for k, pv := range props {
				ps := schemaFromAny(pv)
				if ps == nil {
					ps = &schema.JSON{}
				}
				js.Properties[k] = ps
			}
		}
		if ap, ok := m["additionalProperties"]; ok {
			js.AdditionalProperties = schemaFromAny(ap)
		}
	}
	return js
}

func applyTypeFromString(js *schema.JSON, t string) {
	ls := strings.ToLower(strings.TrimSpace(t))
	switch {
	case ls == "dict" || ls == "object":
		js.Type = schema.Object
	case ls == "array" || strings.Contains(ls, "list"):
		js.Type = schema.Array
		js.Items = &schema.JSON{}
	case ls == "integer" || ls == "int":
		js.Type = schema.Integer
	case ls == "number" || ls == "float":
		js.Type = schema.Number
	case ls == "boolean" || ls == "bool":
		js.Type = schema.Boolean
	case ls == "string":
		js.Type = schema.String
	default:
		// leave empty (permissive)
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func httpErr(w http.ResponseWriter, err error, status int) {
	writeJSON(w, status, map[string]any{"error": err.Error()})
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

// setupHttpLangfuse reads the .env and wires a direct HTTP connection to localhost:3000
func setupHttpLangfuse(ctx context.Context) (*sdktrace.TracerProvider, error) {
	_ = godotenv.Load(".env")
	pubKey := os.Getenv("LANGFUSE_PUBLIC_KEY")
	secKey := os.Getenv("LANGFUSE_SECRET_KEY")
	host := os.Getenv("LANGFUSE_BASE_URL")
	fmt.Println("host:", host)
	if pubKey == "" || secKey == "" || host == "" {
		fmt.Errorf("Missing LANGFUSE_PUBLIC_KEY or LANGFUSE_SECRET_KEY in .env")
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
		fmt.Errorf("Failed to create HTTP exporter: %v", err)
	}

	// Create the Tracer Provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("langfuse-ping-test"),
		)),
	)
	otel.SetTracerProvider(tp)
	return tp, nil
}

func benchSampleID(r *http.Request) string {
	if v := strings.TrimSpace(r.Header.Get("X-Test-Id")); v != "" {
		return v
	}
	return fmt.Sprintf("%d-%s", time.Now().UnixMilli(), strings.ReplaceAll(r.RemoteAddr, ":", "_"))
}

func truncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "...(truncated)"
}

func toolsDigest(rawTools []any) string {
	b, _ := json.Marshal(rawTools)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:8])
}
