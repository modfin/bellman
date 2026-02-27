package nestful

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/dop251/goja"
	"github.com/modfin/bellman"
	"github.com/modfin/bellman/models/gen"
	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/schema"
	"github.com/modfin/bellman/tools"
	"github.com/modfin/bellman/tools/ptc"
)

// --- NESTFUL benchmark adapter (single-shot, with/without PTC) ---

type NestfulBenchmarkRequest struct {
	Model              string  `json:"bellman_model"` // provider/name
	Query              string  `json:"query"`
	Tools              []any   `json:"tools"`
	Temperature        float64 `json:"temperature"`
	MaxTokens          int     `json:"max_tokens"`
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

// Regex to find invalid tool-name characters.
var invalidNameChars = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

func NesfulHandlerFromEnv() http.HandlerFunc {
	bellmanURL := os.Getenv("BELLMAN_URL")
	bellmanToken := os.Getenv("BELLMAN_TOKEN")

	client := bellman.New(bellmanURL, bellman.Key{Name: "nestful", Token: bellmanToken})
	model := "OpenAI/gpt-4o-mini"
	defaultModelFQN := os.Getenv(model)

	return NestfulHandlerWrapper(client, defaultModelFQN)
}

// NestfulHandler exposes a single-shot endpoint that returns predicted tool-call sequences in NESTFUL's format.
//
// - If EnablePTC=false: reads res.Tools and returns [{name,arguments,label}, ...]
// - If EnablePTC=true: expects code_execution; runs JS in goja with interceptors and returns the extracted sequence.
//
// Tools are never executed.

func NestfulHandler(w http.ResponseWriter, r *http.Request, client *bellman.Bellman, defaultModelFQN string) {

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

	if strings.TrimSpace(defaultModelFQN) == "" {
		defaultModelFQN = "OpenAI/gpt-4o-mini"
	}
	model, err := parseModelFQN(defaultModelFQN)
	if err != nil {
		httpErr(w, fmt.Errorf("invalid model: %w", err), http.StatusBadRequest)
		return
	}

	parsedTools, nameMap, outKeysByTool, err := parseNestfulTools(req.Tools)
	if err != nil {
		httpErr(w, fmt.Errorf("invalid tools: %w", err), http.StatusBadRequest)
		return
	}
	for i := range parsedTools {
		parsedTools[i].UsePTC = req.EnablePTC
		// Never executed; just to keep tool refs non-nil.
		parsedTools[i].Function = func(ctx context.Context, call tools.Call) (string, error) { return "{}", nil }
	}
	llm := client.Generator().
		Model(model).
		System(req.SystemPrompt).
		SetTools(parsedTools...).
		SetPTCLanguage(tools.JavaScript).
		Temperature(req.Temperature).
		MaxTokens(req.MaxTokens)

	switch choice {
	case "required":
		llm = llm.SetToolConfig(tools.RequiredTool)
	case "auto":
		llm = llm.SetToolConfig(tools.AutoTool)
	case "none":
		llm = llm.SetToolConfig(tools.NoTool)
	default:
		httpErr(w, fmt.Errorf("invalid tool_choice: %q", req.ToolChoice), http.StatusBadRequest)
		return
	}

	res, err := llm.Prompt(prompt.AsUser(req.Query))
	println("LMM resp", res, err)
	if err != nil {
		httpErr(w, fmt.Errorf("upstream error: %w", err), http.StatusBadGateway)
		return
	}

	generated, content := nestfulGeneratedText(res, parsedTools, nameMap, outKeysByTool, req.JSExtractTimeoutMs)
	writeJSON(w, http.StatusOK, NestfulBenchmarkResponse{
		GeneratedText: generated,
		Content:       content,
		InputTokens:   res.Metadata.InputTokens,
		OutputTokens:  res.Metadata.OutputTokens,
		TotalTokens:   res.Metadata.TotalTokens,
	})
}

func NestfulHandlerWrapper(client *bellman.Bellman, defaultModelFQN string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		NestfulHandler(w, r, client, defaultModelFQN)
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

		parsed = append(parsed, tools.Tool{
			Name:           sanitized,
			Description:    strings.TrimSpace(def.Description + "\nOutput keys: " + strings.Join(outKeys, ", ")),
			ArgumentSchema: s,
		})
	}
	return parsed, nameMap, outKeysByTool, nil
}

func nestfulGeneratedText(res *gen.Response, availableTools []tools.Tool, nameMap map[string]string, outKeysByTool map[string][]string, timeoutMs int) (generated string, content string) {
	if !res.IsTools() {
		text, _ := res.AsText()
		return "[]", text
	}
	out := make([]map[string]any, 0)
	errMsgs := make([]string, 0, 1)
	for _, tc := range res.Tools {
		if tc.Name == "code_execution" {
			var codeArgs struct {
				Code string `json:"code"`
			}
			if err := json.Unmarshal(tc.Argument, &codeArgs); err != nil {
				errMsgs = append(errMsgs, fmt.Sprintf("code_execution args unmarshal error: %v", err))
				continue
			}
			seq, errMsg := executeAndExtractNestful(codeArgs.Code, availableTools, outKeysByTool, timeoutMs)
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

func executeAndExtractNestful(jsCode string, availableTools []tools.Tool, outKeysByTool map[string][]string, timeoutMs int) ([]map[string]any, string) {
	vm := goja.New()
	var captured []map[string]any

	guarded, guardErr := ptc.GuardRailJS(jsCode)
	if guardErr != nil {
		return captured, fmt.Sprintf("code_execution guardrail error: %v", guardErr)
	}

	timer := time.AfterFunc(time.Duration(timeoutMs)*time.Millisecond, func() {
		vm.Interrupt("timeout")
	})
	defer timer.Stop()

	for _, t := range availableTools {
		tName := t.Name
		keys := outKeysByTool[tName]
		if len(keys) == 0 {
			keys = []string{"result"}
		}
		interceptor := func(call goja.FunctionCall) goja.Value {
			// Reserve the label index for this tool call so the returned placeholder
			// matches the final label numbering ($var_1, $var_2, ...).
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
					argsMap["arg_0"] = first
					for i := 1; i < len(call.Arguments); i++ {
						argsMap[fmt.Sprintf("arg_%d", i)] = call.Arguments[i].Export()
					}
				}
			}

			_ = normalizeVarRefs(argsMap)
			captured = append(captured, map[string]any{"name": tName, "arguments": argsMap})

			// Return a JS object so the model can chain on declared output keys.
			return vm.ToValue(outObj)
		}
		_ = vm.Set(tName, interceptor)
	}

	if _, err := vm.RunString(guarded); err != nil {
		return captured, fmt.Sprintf("code_execution run error: %v", err)
	}
	fmt.Println("Guarded", guarded)
	fmt.Println("captured", captured)
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

func parseModelFQN(fqn string) (gen.Model, error) {
	fqn = strings.TrimSpace(fqn)
	provider, name, found := strings.Cut(fqn, "/")
	if !found {
		provider, name, found = strings.Cut(fqn, ".")
	}
	if !found {
		return gen.Model{}, fmt.Errorf("expected provider/name (or provider.name), got %q", fqn)
	}
	provider = canonicalProvider(provider)
	name = canonicalModelName(name)
	if provider == "" || name == "" {
		return gen.Model{}, fmt.Errorf("expected provider/name (or provider.name), got %q", fqn)
	}
	return gen.Model{Provider: provider, Name: name}, nil
}
func canonicalProvider(p string) string {
	pl := strings.ToLower(strings.TrimSpace(p))
	switch pl {
	case "openai":
		return "OpenAI"
	case "vertexai", "vertex":
		return "VertexAI"
	case "anthropic":
		return "Anthropic"
	case "ollama":
		return "Ollama"
	case "vllm":
		return "vLLM"
	case "voyageai", "voyage":
		return "VoyageAI"
	default:
		return strings.TrimSpace(p)
	}
}

func canonicalModelName(n string) string {
	n = strings.TrimSpace(n)
	n = strings.ReplaceAll(n, "_", "-")
	if strings.HasPrefix(n, "gpt4o-") {
		n = "gpt-4o-" + strings.TrimPrefix(n, "gpt4o-")
	}
	return n
}
