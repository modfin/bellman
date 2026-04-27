package tools

import (
	"context"
	"github.com/modfin/bellman/schema"
)

type EmptyArgs struct{}

// ToolChoice represents a tool selection configuration for the model.
// It is distinct from Tool — it only carries a name used to control tool behavior.
type ToolChoice struct {
	Name string `json:"name"`
}

// NoTool means the model will not call any tool and instead generates a message
var NoTool = ToolChoice{Name: "none"}

// AutoTool means the model can pick between generating a message or calling one or more tools
var AutoTool = ToolChoice{Name: "auto"}

// RequiredTool means the model must call one or more tools.
var RequiredTool = ToolChoice{Name: "required"}

var ControlTools = []ToolChoice{
	NoTool,
	AutoTool,
	RequiredTool,
}

type ToolOption func(tool Tool) Tool

type Function func(ctx context.Context, call Call) (response string, err error)

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
		tool.ArgumentSchema = schema.From(arg)
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
	Name           string                                               `json:"name"`
	Description    string                                               `json:"description"`
	ArgumentSchema *schema.JSON                                         `json:"argument_schema,omitempty"`
	Function       func(ctx context.Context, call Call) (string, error) `json:"-"`
}

type Call struct {
	ID       string `json:"id,omitempty"`
	Name     string `json:"name"`
	Argument []byte `json:"argument"`

	Ref *Tool `json:"-"`
}
