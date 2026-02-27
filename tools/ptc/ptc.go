package ptc

import (
	"fmt"
	"sync"

	"github.com/dop251/goja"
	"github.com/modfin/bellman/tools"
)

type Runtime struct {
	Mutex sync.Mutex // Only one operator at a time --> prevent unexpected concurrency runtime errors
	JS    *goja.Runtime
	// Python *python.Environment <-- other code exec vm envs.
}

const (
	CodeExecutionToolName string = "code_execution"
)

// IsPTCEnabled checks if there is a tool with PTC enabled (as PTC is set per tool)
func IsPTCEnabled(inputTools []tools.Tool) bool {
	for _, t := range inputTools {
		if t.UsePTC {
			return true
		}
	}
	return false
}

// ExtractPTCTools separates regular tools from PTC tools and returns both slices
func ExtractPTCTools(inputTools []tools.Tool) ([]tools.Tool, []tools.Tool) {
	var regularTools []tools.Tool
	var ptcTools []tools.Tool

	// get PTC enabled tools
	for _, t := range inputTools {
		if t.UsePTC {
			ptcTools = append(ptcTools, t)
		} else {
			regularTools = append(regularTools, t)
		}
	}
	return regularTools, ptcTools
}

// AdaptToolsToPTC converts a list of Bellman tools into a single PTC tool with code execution environment
func AdaptToolsToPTC(runtime *Runtime, ptcTools []tools.Tool, language tools.ProgramLanguage) (tools.Tool, string, error) {
	switch language {
	case tools.JavaScript:
		return adaptToolsToJSPTC(runtime, ptcTools)
	case tools.Python:
		return tools.Tool{}, "", fmt.Errorf("ptc python not implemented")
	case tools.Go:
		return tools.Tool{}, "", fmt.Errorf("ptc go not implemented")
	case tools.Lua:
		return tools.Tool{}, "", fmt.Errorf("ptc lua not implemented")
	default: // default to JS
		return adaptToolsToJSPTC(runtime, ptcTools)
	}
}
