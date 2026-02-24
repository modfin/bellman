package tools

import (
	"context"

	"github.com/modfin/bellman/schema"
)

// ToolMode represents how the model should select tools
type ToolMode string

const (
	ToolModeNone     ToolMode = "none"
	ToolModeAuto     ToolMode = "auto"
	ToolModeRequired ToolMode = "required"
	ToolModeSpecific ToolMode = "specific"
)

// ToolConfig controls how the model selects tools
type ToolConfig struct {
	Mode ToolMode `json:"-"`    // Not serialized - tracks intent
	Name string   `json:"name"` // Serialized for wire format
}

// ToolNone prevents the model from calling any tool
func ToolNone() *ToolConfig {
	return &ToolConfig{Mode: ToolModeNone, Name: "none"}
}

// ToolAuto lets the model decide whether to use tools
func ToolAuto() *ToolConfig {
	return &ToolConfig{Mode: ToolModeAuto, Name: "auto"}
}

// ToolRequired forces the model to call at least one tool
func ToolRequired() *ToolConfig {
	return &ToolConfig{Mode: ToolModeRequired, Name: "required"}
}

// ToolSpecific forces the model to call a specific tool
func ToolSpecific(tool *Tool) *ToolConfig {
	return &ToolConfig{Mode: ToolModeSpecific, Name: tool.Name}
}

type ToolOption func(*Tool) *Tool

type Function func(ctx context.Context, call Call) (response string, err error)

func WithDescription(description string) ToolOption {
	return func(tool *Tool) *Tool {
		tool.Description = description
		return tool
	}
}

func WithFunction(callback Function) ToolOption {
	return func(tool *Tool) *Tool {
		tool.Function = callback
		return tool
	}
}

func WithArgSchema(arg any) ToolOption {
	return func(tool *Tool) *Tool {
		tool.ArgumentSchema = schema.From(arg)
		return tool
	}
}

func NewTool(name string, options ...ToolOption) *Tool {
	t := &Tool{
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

// clone creates a deep copy of the Tool
func (t *Tool) clone() *Tool {
	if t == nil {
		return nil
	}
	tt := &Tool{
		Name:        t.Name,
		Description: t.Description,
		Function:    t.Function,
	}
	if t.ArgumentSchema != nil {
		cp := *t.ArgumentSchema
		tt.ArgumentSchema = &cp
	}
	return tt
}

// SetDescription sets the description of the tool
func (t *Tool) SetDescription(description string) *Tool {
	tt := t.clone()
	tt.Description = description
	return tt
}

// SetFunction sets the function handler of the tool
func (t *Tool) SetFunction(callback Function) *Tool {
	tt := t.clone()
	tt.Function = callback
	return tt
}

// SetArgSchema sets the argument schema of the tool
func (t *Tool) SetArgSchema(arg any) *Tool {
	tt := t.clone()
	tt.ArgumentSchema = schema.From(arg)
	return tt
}

type Call struct {
	ID       string `json:"id,omitempty"`
	Name     string `json:"name"`
	Argument []byte `json:"argument"`

	Ref *Tool `json:"-"`
}
