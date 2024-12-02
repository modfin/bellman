package bellman

import (
	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/tools"
)

type LLM interface {
	Generator(options ...GeneratorOption) Generator
}

type Prompter interface {
	SetConfig(config Config)
	Prompt(prompts ...prompt.Prompt) (Response, error)
}

type Response interface {
	// AsText will return the response as a string and an error if no response exist
	// is the response is json, it will be present in this string
	AsText() (string, error)

	// AsTools will return the name of the tool to use, the argument to pass to the tool, in json format form specified schema, and an error if the response is not a tool
	AsTools() ([]tools.Call, error)

	// Eval will run the callback associated with a tool response, otherwise it will return an error
	Eval() (err error)

	// Unmarshal will unmarshal the response into the provided reference
	Unmarshal(ref any) error
}
