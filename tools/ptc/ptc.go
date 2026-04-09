package ptc

import (
	"context"
	"fmt"

	"github.com/modfin/bellman/tools"
	"github.com/modfin/bellman/tools/ptc/js"
)

type Runtime interface {
	AdaptTools(tools ...tools.Tool) (tools.Tool, error)
	Guardrail(code string) (string, error)
	SystemFragment(tool ...tools.Tool) (string, error)
	Lock()
	Unlock()
	Execute(ctx context.Context, code string) (string, error, error)
}

type ProgramLanguage string

const (
	JavaScript ProgramLanguage = "javascript"
	Python     ProgramLanguage = "python"
	Lua        ProgramLanguage = "lua"
)

const (
	ToolName string = "code_execution"
)

func NewRuntime(lang ProgramLanguage) (Runtime, error) {
	switch lang {
	case JavaScript:
		return js.NewRuntime(ToolName)
	}
	return nil, fmt.Errorf("language unsupported: %s", lang)
}

// SplitTools separates regular tools from PTC tools and returns both slices
func SplitTools(inputTools []tools.Tool) ([]tools.Tool, []tools.Tool) {
	var regularTools []tools.Tool
	var ptcTools []tools.Tool

	for _, t := range inputTools {
		if t.UsePTC {
			ptcTools = append(ptcTools, t)
			continue
		}
		regularTools = append(regularTools, t)
	}
	return regularTools, ptcTools
}
