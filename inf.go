package bellman

import (
	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/tools"
)

type LLM interface {
	Generator(options ...GeneratorOption) Generator
}

type Generator interface {

	// Model lets you specify which model to use
	Model(model GenModel) Generator

	// System lets you specify a system prompt, ie, the prompt that the model will use to generate a response
	System(prompt string) Generator

	// Output lets you specify the output schema, eg for unmarshalling reuslt into a struct.
	Output(element any) Generator

	// Tools lets you specifics a selection of tools that the ai may use
	Tools(tool ...tools.Tool) Generator

	// Tool lets you specifics which tool to use, eg, ai.NoTool, ai.AutoTool, ai.RequiredTool or a specific tool
	Tool(tool tools.Tool) Generator

	StopAt(stop ...string) Generator
	Temperature(temperature float64) Generator
	TopP(topP float64) Generator
	MaxTokens(maxTokens int) Generator

	Prompt(prompt ...prompt.Prompt) (Response, error)
}

type Response interface {

	// AsText will return the response as a string and an error if no response exist
	// is the response is json, it will be present in this string
	AsText() (string, error)

	// AsTools will return the name of the tool to use, the argument to pass to the tool, in json format form specified schema, and an error if the response is not a tool
	AsTools() ([]ToolCallback, error)

	// Eval will run the callback associated with a tool response, otherwise it will return an error
	Eval() (err error)

	// Unmarshal will unmarshal the response into the provided reference
	Unmarshal(ref any) error
}

type ToolCallback struct {
	Name     string
	Argument string
}
