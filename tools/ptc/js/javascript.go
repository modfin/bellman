package js

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dop251/goja"
	"github.com/modfin/bellman/schema"
	"github.com/modfin/bellman/tools"
)

type JavaScript struct {
	runtime  *goja.Runtime
	mu       sync.Mutex
	toolName string
	console  *ConsoleOutput
	Log      *slog.Logger `json:"-"`
}

func NewRuntime(toolName string) *JavaScript {
	javaScript := &JavaScript{
		runtime:  goja.New(),
		mu:       sync.Mutex{},
		toolName: toolName,
	}
	return javaScript.registerConsole()
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

	// tool documentation fragment
	fragment := docsFragment(tool...)

	// create the final PTC tool
	ptcTool := tools.NewTool(j.toolName,
		tools.WithDescription(
			strings.ReplaceAll(`Execute top-level JavaScript in a persistent Goja runtime to call available Tool Functions.

RETURN: Always end with an explicit 'return' statement.
  return { a, b };          ✓
  return result;            ✓
  var x = result;           ✗  (returns nothing)

RULES:
- Synchronous only. No async/await.
- Variables persist. Use 'var', do not redeclare let/const.
- ONE script per turn. Batch all independent calls.
- Never call the same Function twice with identical arguments.

REPL exception: only yield if a Function returns /* Unknown Schema */
AND the next Function strictly requires a field from that result.

Available Tool Functions:

{fragment}
`, "{fragment}", fragment),
		),
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
		// TODO: pass real context if available
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
	j.console.Last = "" // reset console output each execution

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

	res, resErr := j.runtime.RunString(code)
	if resErr != nil {
		return "", resErr, nil
	}

	var jsonBytes []byte
	if res == nil || goja.IsUndefined(res) {
		return nilValue, nil, nil
	}

	jsonBytes, err = json.Marshal(res.Export())
	if err != nil {
		return "", nil, err
	}
	result := string(jsonBytes)

	if j.console.Last != "" && result == nilValue { // if no return expression; fallback to last console.log
		return j.console.Last, nil, nil
	}

	return string(jsonBytes), nil, nil
}

type ConsoleOutput struct {
	Last string
}

func (j *JavaScript) registerConsole() *JavaScript {
	out := &ConsoleOutput{}

	makeLogger := func(level string) func(goja.FunctionCall) goja.Value {
		return func(call goja.FunctionCall) goja.Value {
			parts := make([]string, len(call.Arguments))
			for i, arg := range call.Arguments {
				parts[i] = fmt.Sprintf("%v", arg.Export())
			}
			msg := strings.Join(parts, " ")
			j.log(msg, "level", level)
			out.Last = msg
			return goja.Undefined()
		}
	}

	console := j.runtime.NewObject()
	console.Set("log", j.runtime.ToValue(makeLogger("log")))
	console.Set("info", j.runtime.ToValue(makeLogger("info")))
	console.Set("warn", j.runtime.ToValue(makeLogger("warn")))
	console.Set("error", j.runtime.ToValue(makeLogger("error")))

	j.runtime.Set("console", console)
	j.runtime.Set("print", j.runtime.ToValue(makeLogger("print")))

	j.console = out
	return j
}

func docsFragment(tool ...tools.Tool) string {
	var descriptions []string
	for _, t := range tool {
		signature := formatToolSignature(t)
		descriptions = append(descriptions, signature)
	}
	return strings.Join(descriptions, "\n\n")
}

type ArgField struct {
	Name     string
	Type     string
	Required bool
}

func formatToolSignature(t tools.Tool) string {
	args := extractArgs(t.ArgumentSchema)

	var fields []string
	for _, a := range args {
		name := a.Name
		if !a.Required {
			name += "?"
		}
		fields = append(fields, fmt.Sprintf("  %s: %s", name, a.Type))
	}

	argBlock := "{}"
	if len(fields) > 0 {
		argBlock = "{\n" + strings.Join(fields, ",\n") + "\n}"
	}

	// get return types - If schema is missing or empty, trigger the REPL warning
	returnType := "unknown"
	jsDocWarning := " /* Unknown Schema */"

	isKnown := true
	if t.ResponseSchema == nil || t.ResponseSchema.Type == "" {
		isKnown = false
	} else if t.ResponseSchema.Type == "object" && len(t.ResponseSchema.Properties) == 0 {
		// Only objects with 0 properties are considered "Unknown"
		isKnown = false
	}
	if isKnown {
		returnType = SchemaToTS(t.ResponseSchema)
		jsDocWarning = ""
	}

	return fmt.Sprintf("/**\n * %s\n * @returns {%s}%s\n */\ndeclare function %s(params: %s): %s;",
		t.Description, returnType, jsDocWarning, t.Name, argBlock, returnType)
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
		args = append(args, ArgField{
			Name:     name,
			Type:     mapJSONSchemaType(prop),
			Required: required[name],
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

	if !strings.Contains(code, "return ") {
		j.log("guardrail no return expression")
		return code, errors.New("runtime error: script must end with an explicit 'return <expression>' statement. variable assignments and bare expressions do not return data")
	}

	// ensure IIFE to enable return expression
	return "(function() {\n" + code + "\n})()", nil
}

// TODO replace with template
func (j *JavaScript) SystemFragment(tool ...tools.Tool) string {
	system := strings.ReplaceAll(`You have access to Programmatic Tool-Calling (PTC).
Use the '{ptc_tool_name}' tool to interact with external data and functions.
Tool calls can be costly — use only when necessary, write compact code.

# JavaScript Runtime (Goja)

Your code runs inside a function body — use 'return' to return the final result.

- Synchronous only. No async/await.
- Variables persist across turns. Use 'var' (do not redeclare let/const).
- You MUST return data explicitly: 'return { a, b };'
- Tool Functions are deterministic. Never call the same Function with identical arguments twice.

## When To Use This Tool

Use '{ptc_tool_name}' ONLY when external Tool Functions are required.
If the request can be answered with reasoning or general knowledge, respond directly.

## Execution Strategy

Default: write ONE complete batch script. Call all independent Functions together.
`+"```javascript"+`
var user = searchUsers({ query: "john" });
var weather = getWeather({ city: "Stockholm" });
return { user, weather };
`+"```"+`

### Exception: REPL Yielding

ONLY yield across turns if:
1. Function A returns /* Unknown Schema */, AND
2. Function B strictly requires a specific field from A's result.

In that case: execute A, return its result, and STOP. Do not guess field names. Wait for the result.

# Finishing

This tool's output is NOT visible to the user.
Once you have the data you need, STOP calling the tool and respond to the user in plain text.
`, "{ptc_tool_name}", j.toolName)

	// create PTC system prompt fragment with tools
	return "\n" + system + "\n## Available JavaScript Tool Functions inside the runtime:\n\n" + docsFragment(tool...)
}

func (j *JavaScript) SetLogger(logger *slog.Logger) *JavaScript {
	j.Log = logger
	return j
}
