package js

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
}

func NewRuntime(toolName string) *JavaScript {
	return &JavaScript{
		runtime:  goja.New(),
		mu:       sync.Mutex{},
		toolName: toolName,
	}
}

func (j *JavaScript) Lock() {
	j.mu.Lock()
}

func (j *JavaScript) Unlock() {
	j.mu.Unlock()
}

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

		code, err := j.Guardrail(arg.Code)
		if err != nil {
			return err.Error(), nil
		}

		return j.Execute(code)
	}

	// tool documentation fragment
	fragment := docsFragment(tool...)

	// create the final PTC tool
	ptcTool := tools.NewTool(j.toolName,
		tools.WithDescription(`Execute top-level JavaScript in a persistent Goja runtime to call available Tool Functions.

Use this tool ONLY when external Tool Functions are required to fetch or interact with data.
The user CANNOT see this tool's output — you must respond to them in normal text output.

DEFAULT USAGE (REQUIRED): Write ONE complete batch script that performs all needed Function calls. Do NOT call Tool Functions one-by-one across turns.

REPL is allowed ONLY if:
- A Function returns /* Unknown Schema */
AND
- Another Function strictly requires a specific field from that result.

RULES:
- At most ONE script per turn.
- Never call the same Function twice with identical arguments.
- Variables persist. Use 'var' or reassign (do not redeclare let/const).
- The LAST evaluated expression is returned automatically. NEVER use 'return;' or var assignment on last line.
- Final line: Variable assignments evaluate to null. Your script MUST end with an object (e.g., '({a, b});') to successfully return data.
- Synchronous only. No async/await or external APIs.

Available JavaScript Tool Functions inside the runtime:`+
			"\n\n"+
			fragment,
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
func (j *JavaScript) Execute(code string) (resString string, err error) {
	j.Lock()
	defer j.Unlock()

	// panic recovery
	defer func() {
		if r := recover(); r != nil {
			log.Printf("error: runtime panic! recovering.")
			resString = fmt.Sprintf(`{"error": "critical runtime panic: %v"}`, r)
			err = nil
		}
	}()

	// timeout interrupt
	timer := time.AfterFunc(10*time.Second, func() {
		log.Printf("error: runtime timeout interrupt!")
		j.runtime.Interrupt("timeout: script execution took too long (possible infinite loop)")
	})
	defer timer.Stop()

	res, err := j.runtime.RunString(code)
	if err != nil {
		return fmt.Sprintf(`{"error": %q}`, err.Error()), nil
	}

	var jsonBytes []byte
	if res == nil || goja.IsUndefined(res) {
		return "null", nil
	}

	jsonBytes, err = json.Marshal(res.Export())
	if err != nil {
		return "", err
	}

	return string(jsonBytes), nil
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
func (j *JavaScript) Guardrail(code string) (string, error) { // TODO: add more/update guardrails
	if code == "" {
		log.Printf("[PTC] Blocked empty code attempt\n")
		errMsg := "no javascript code provided. validate tool input arguments, required format: '{\"code\": string}'." // TODO: string format
		return code, fmt.Errorf("error: %s", errMsg)
	}

	if strings.Contains(code, "print( ") || strings.Contains(code, "console.log(") {
		log.Printf("[PTC] Blocked log attempt\n")
		errMsg := "RuntimeError: Log functions (e.g., 'console.log' or 'print') are strictly FORBIDDEN in this environment. You must use return data via the function return only. Rewrite the code immediately."
		return code, fmt.Errorf("error: %s", errMsg)
	}

	if strings.Contains(code, "async ") || strings.Contains(code, "await") || strings.Contains(code, "async(") {
		log.Printf("[PTC] Blocked async code attempt\n")
		errMsg := "RuntimeError: Async functions are strictly FORBIDDEN in this environment. You must use synchronous, blocking calls (e.g., 'const x = tool()', NOT 'await tool()'). Rewrite the code immediately."
		return code, fmt.Errorf("error: %s", errMsg)
	}
	return code, nil
}

// TODO replace with template
func (j *JavaScript) SystemFragment(tool ...tools.Tool) string {
	system := strings.ReplaceAll(`Your are an LLM-based AI Agent enhanced with Programmatic Tool-Calling (PTC).
The PTC tool at your disposal is the '{ptc_tool_name}' tool, use it to interact with data!

Tool calls can be costly, use only when necessary to fetch or interact with data, and write compact code.

# JavaScript runtime (Goja) - Accessible through '{ptc_tool_name}' Tool

- Write standard top-level runtime. No async/await, no logging.
- Variables persist across turns. Use 'var' (do not redeclare let/const).
- The LAST evaluated expression is returned automatically. NEVER use 'return;' or var assignment on last line.
- Final line: Variable assignments evaluate to null. Your script MUST end with an object (e.g., '({a, b});') to successfully return data.
- Tool Functions are deterministic. NEVER call a Function twice with identical arguments. Read your history.

## When To Use This Tool
Use '{ptc_tool_name}' ONLY if external Tool Functions are required.
If the request can be answered with reasoning or general knowledge → respond user directly in plain text (do NOT call the tool).

## Default Execution Strategy — SINGLE BATCH (Required)
Before writing code, determine if intermediate outputs are strictly required. 
Your default behavior MUST be to write the entire solution in a SINGLE script. Batch all independent Function calls together.

Example (Correct Default Strategy):
var user = searchUsers({ query: "john" }); // returns 'unknown'
var emailSent = sendEmail({ body: "Hello, let's meet." }); // returns 'boolean'
({user, emailSent}); // Returns both results in a single object. No need to yield/pause.

### The ONLY Exception: REPL Yielding
Use REPL IF AND ONLY IF:
1) Function A returns /* Unknown Schema */, AND
2) Another Function B strictly requires a specific field from A’s result.

Yield control (STOP) IF AND ONLY IF Function A has an /* Unknown Schema */ AND Function B strictly requires a property from Function A's output.
Execute Function A, put its result on the last line, and STOP. DO NOT guess property names.

## Finishing the Task (CRITICAL)
This tool ONLY fetches and interacts with data. The user CANNOT see the output of this tool. 
When you have the final answer, you MUST STOP using '{ptc_tool_name}'. 
To finish, YOU MUST write a normal, plain-text conversational response to the user.

Do NOT call the tool again unless new information is required.

# Respond the User
When you have completed the task, you MUST respond the users request directly in text!
`, "{ptc_tool_name}", j.toolName)

	// create PTC system prompt fragment with tools
	return "\n" + system + "\n## Available JavaScript Tool Functions inside the runtime:\n\n" + docsFragment(tool...)
}
