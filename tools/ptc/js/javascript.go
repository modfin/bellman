package js

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/dop251/goja"
	"github.com/modfin/bellman/schema"
	"github.com/modfin/bellman/tools"
)

type JavaScript struct {
	runtime  *goja.Runtime
	mu       sync.Mutex
	toolName string
	output   *resultOutput
	Log      *slog.Logger `json:"-"`
}

type TemplateData struct {
	PTCToolName string
	Signatures  []FunctionSignatureData
}

type FunctionSignatureData struct {
	Name          string
	Description   string
	Args          []ArgField
	ReturnType    string
	UnknownSchema bool
}

type ArgField struct {
	Name        string
	Type        string
	Required    bool
	Description string
}

//go:embed prompts.tmpl
var templateFS embed.FS

var parsedTemplates *template.Template

func init() {
	var err error
	parsedTemplates, err = template.ParseFS(templateFS, "prompts.tmpl")
	if err != nil {
		panic(fmt.Errorf("failed to parse prompts.tmpl: %w", err))
	}
}

func NewRuntime(toolName string) (*JavaScript, error) {
	javaScript := &JavaScript{
		runtime:  goja.New(),
		mu:       sync.Mutex{},
		toolName: toolName,
	}
	return javaScript.registerResult()
}

func (j *JavaScript) Lock() {
	j.mu.Lock()
}

func (j *JavaScript) Unlock() {
	j.mu.Unlock()
}

func (j *JavaScript) Runtime() *goja.Runtime {
	return j.runtime
}

func (j *JavaScript) log(msg string, args ...any) {
	if j.Log == nil {
		return
	}
	j.Log.Debug("[bellman/javascript] "+msg, args...)
}

const nilValue string = "null" // nil in JS

// AdaptTools converts a list of Bellman tools into a single PTC tool with runtime execution environment
func (j *JavaScript) AdaptTools(tool []tools.Tool) (tools.Tool, error) {
	for _, t := range tool {
		err := j.bindToolFunction(t)
		if err != nil {
			return tools.Tool{}, fmt.Errorf("error adapting tools to ptc: %w", err)
		}
	}

	type CodeArgs struct {
		Code string `json:"code" json-description:"The executable top-level JavaScript code string."`
	}
	executor := func(ctx context.Context, call tools.Call) (string, error) {
		var arg CodeArgs
		if err := json.Unmarshal(call.Argument, &arg); err != nil {
			return "", err
		}

		res, resErr, err := j.Execute(arg.Code)
		if err != nil {
			return res, err
		}

		// return error string to LLM
		if resErr != nil {
			return fmt.Sprintf(`{"error": %q}`, resErr.Error()), err
		}

		return res, err
	}

	// create tool description
	sigs := functionSignatures(tool...)

	data := TemplateData{
		PTCToolName: j.toolName,
		Signatures:  sigs,
	}
	var buf bytes.Buffer
	if err := parsedTemplates.ExecuteTemplate(&buf, "ptc_tool_description", data); err != nil {
		return tools.Tool{}, fmt.Errorf("failed to execute tool description template: %w", err)
	}
	toolDescription := buf.String()

	// create the final PTC tool
	ptcTool := tools.NewTool(j.toolName,
		tools.WithDescription(toolDescription),
		tools.WithArgSchema(CodeArgs{}),
		tools.WithFunction(executor),
	)

	return ptcTool, nil
}

// bindToolFunction wraps a Bellman tool as a runtime function: toolName({ args... })
func (j *JavaScript) bindToolFunction(tool tools.Tool) error {
	wrapper := func(call goja.FunctionCall) goja.Value {
		// check if LLM passed multiple arguments (common mistake)
		if len(call.Arguments) > 1 {
			errMsg := fmt.Sprintf("Error: %s expects a single configuration object argument, but received %d arguments. Usage: %s({ key: val })",
				tool.Name, len(call.Arguments), tool.Name)
			return j.runtime.ToValue(map[string]string{"error": errMsg})
		}

		// extract runtime argument (expecting a single object)
		if len(call.Arguments) == 0 {
			return j.runtime.NewGoError(fmt.Errorf("tool %s requires arguments", tool.Name))
		}
		jsArgs := call.Argument(0).Export()

		// marshal args to JSON for the Bellman tool
		jsonArgs, err := json.Marshal(jsArgs)
		if err != nil {
			return j.runtime.NewGoError(err)
		}

		// execute the actual go tool
		res, err := tool.Function(context.Background(), tools.Call{
			Argument: jsonArgs,
		})
		if err != nil {
			// return error string directly so the LLM can self-correct, e.g., "json: cannot unmarshal number..."
			return j.runtime.ToValue(map[string]any{"ok": false, "error": err.Error()})
		}

		// unmarshal result back to runtime object if possible
		var parsed interface{}
		if err := json.Unmarshal([]byte(res), &parsed); err == nil {
			return j.runtime.ToValue(parsed)
		}

		// otherwise return raw string
		return j.runtime.ToValue(res)
	}

	j.Lock()
	defer j.Unlock()

	err := j.runtime.Set(tool.Name, wrapper)
	if err != nil {
		return err
	}

	return nil
}

// Execute runs a code script in the runtime, uses same error handling as LLM (runtime errors return string!)
func (j *JavaScript) Execute(code string) (resString string, resErr error, err error) {
	code, resErr = j.Guardrail(code)
	if resErr != nil {
		return "", resErr, nil
	}
	j.output.set = false // reset output

	j.Lock()
	defer j.Unlock()

	// panic recovery
	defer func() {
		if r := recover(); r != nil {
			j.log("error: runtime panic! recovering.")
			resErr = fmt.Errorf("critical runtime panic: %v", r)
			err = nil
		}
	}()

	// timeout interrupt
	timer := time.AfterFunc(10*time.Second, func() {
		j.log("error: runtime timeout interrupt!")
		j.runtime.Interrupt("timeout: script execution took too long (possible infinite loop)")
	})
	defer timer.Stop()

	// set non-fresh on execution!
	j.fresh = false

	_, resErr = j.runtime.RunString(code)
	if resErr != nil {
		return "", resErr, nil
	}

	// if result(); used, return the value
	if j.output.set {
		return j.output.value, nil, nil
	}

	return nilValue, nil, nil
}

type resultOutput struct {
	value string
	set   bool
}

func (j *JavaScript) registerResult() (*JavaScript, error) {
	out := &resultOutput{}
	j.output = out

	err := j.runtime.Set("result", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return goja.Undefined()
		}
		b, err := json.Marshal(call.Argument(0).Export())
		if err != nil {
			return goja.Undefined()
		}
		out.value = string(b)
		out.set = true
		return goja.Undefined()
	})
	if err != nil {
		return nil, err
	}

	return j, nil
}

// Guardrail guardrails code before exec; important since LLMs trained for diff. coding objectives
func (j *JavaScript) Guardrail(code string) (string, error) {
	if code == "" {
		j.log("guardrail empty code")
		return code, errors.New("no javascript code provided. validate tool input arguments, required format: '{\"code\": string}'")
	}

	if strings.Contains(code, "async ") || strings.Contains(code, "await") || strings.Contains(code, "async(") {
		j.log("guardrail async code")
		return code, errors.New("runtime error: async functions are unavailable in this runtime. must use synchronous, blocking calls (e.g., 'var x = tool()')")
	}

	if strings.Contains(code, "console.log(") || strings.Contains(code, "print(") {
		j.log("guardrail console/print usage")
		return code, errors.New("runtime error: console.log() and print() are not for returning data. use result(value) to return data")
	}

	if !strings.Contains(code, "result(") {
		j.log("guardrail missing result()")
		return code, errors.New("runtime error: script must call result(value) exactly once to return data. example: result({ a, b })")
	}

	return code, nil
}

	j.console = out
	return j
}

	if err := parsedTemplates.ExecuteTemplate(&buf, "ptc_system_prompt", data); err != nil {
		j.log("failed to execute system prompt template", "error", err)
		return "", err
	}

	return buf.String(), nil
}

func functionSignatures(tool ...tools.Tool) []FunctionSignatureData {
	var signatures []FunctionSignatureData
	for _, t := range tool {
		// Figure out return type and if schema is unknown
		returnType := "unknown"
		unknownSchema := true

		if t.ResponseSchema != nil && t.ResponseSchema.Type != "" {
			if !(t.ResponseSchema.Type == "object" && len(t.ResponseSchema.Properties) == 0) {
				returnType = SchemaToTS(t.ResponseSchema)
				unknownSchema = false
			}
		}

		signatures = append(signatures, FunctionSignatureData{
			Name:          t.Name,
			Description:   t.Description,
			Args:          extractArgs(t.ArgumentSchema),
			ReturnType:    returnType,
			UnknownSchema: unknownSchema,
		})
	}
	return signatures
}

func extractArgs(s *schema.JSON) []ArgField {
	if s == nil || len(s.Properties) == 0 {
		return nil
	}

	required := map[string]bool{}
	for _, r := range s.Required {
		required[r] = true
	}

	var args []ArgField
	for name, prop := range s.Properties {
		cleanDesc := strings.ReplaceAll(prop.Description, "\n", " ")
		cleanDesc = strings.TrimSpace(cleanDesc)

		args = append(args, ArgField{
			Name:        name,
			Type:        mapJSONSchemaType(prop),
			Required:    required[name],
			Description: cleanDesc,
		})
	}

	sort.Slice(args, func(i, j int) bool {
		return args[i].Name < args[j].Name
	})

	return args
}

func mapJSONSchemaType(s *schema.JSON) string {
	if s == nil {
		return "unknown"
	}

	switch s.Type {
	case "string":
		return "string"
	case "number", "integer":
		return "number"
	case "boolean":
		return "boolean"
	case "array":
		return "any[]"
	case "object":
		return "object"
	default:
		return "unknown"
	}
}

// SchemaToTS recursively converts a bellman schema.JSON into a TypeScript type string
func SchemaToTS(s *schema.JSON) string {
	if s == nil {
		return "any"
	}

	switch s.Type {
	case "string":
		return "string"
	case "integer", "number":
		return "number"
	case "boolean":
		return "boolean"
	case "array":
		// Assuming schema.JSON has an Items field for array types
		if s.Items != nil {
			return fmt.Sprintf("%s[]", SchemaToTS(s.Items))
		}
		return "any[]"
	case "object":
		if len(s.Properties) > 0 {
			var builder strings.Builder
			builder.WriteString("{ ")

			// Create a quick lookup map for required fields
			reqMap := make(map[string]bool)
			for _, r := range s.Required {
				reqMap[r] = true
			}

			// Sort keys for deterministic output
			keys := make([]string, 0, len(s.Properties))
			for k := range s.Properties {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			for _, key := range keys {
				prop := s.Properties[key]
				opt := "?"
				if reqMap[key] {
					opt = ""
				}
				builder.WriteString(fmt.Sprintf("%s%s: %s; ", key, opt, SchemaToTS(prop)))
			}
			builder.WriteString("}")
			return builder.String()
		}
		return "Record<string, any>"
	default:
		return "any"
	}
}

func (j *JavaScript) SetLogger(logger *slog.Logger) *JavaScript {
	j.Log = logger
	return j
}
