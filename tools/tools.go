package tools

import (
	"github.com/modfin/bellman/schema"
)

type EmptyArgs struct{}

// NoTool means the model will not call any tool and instead generates a message
var NoTool = Tool{
	Name: "none",
}

// AutoTool means the model can pick between generating a message or calling one or more tools
var AutoTool = Tool{
	Name: "auto",
}

// RequiredTool means the model must call one or more tools.
var RequiredTool = Tool{
	Name: "required",
}

var ControlTools = []Tool{
	NoTool,
	AutoTool,
	RequiredTool,
}

type ToolOption func(tool Tool) Tool

type Function func(jsonArgument string) (response string, err error)

func WithDescription(description string) ToolOption {
	return func(tool Tool) Tool {
		tool.Description = description
		return tool
	}
}

func WithFunction(callback Function) ToolOption {
	return func(tool Tool) Tool {
		tool.Function = callback
		return tool
	}
}

func WithArgSchema(arg any) ToolOption {
	return func(tool Tool) Tool {
		tool.ArgumentSchema = schema.New(arg)
		return tool
	}
}

func NewTool(name string, options ...ToolOption) Tool {
	t := Tool{
		Name: name,
	}
	for _, opt := range options {
		t = opt(t)
	}
	return t
}

type Tool struct {
	Name           string
	Description    string
	ArgumentSchema *schema.JSON
	Function       func(jsonArg string) (string, error)
}

type Call struct {
	Name     string
	Argument string

	Ref *Tool
}
