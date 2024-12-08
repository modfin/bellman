package gen

import (
	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/schema"
	"github.com/modfin/bellman/tools"
)

type Request struct {
	Model Model `json:"model"`

	SystemPrompt string `json:"system_prompt"`

	StopSequences []string `json:"stop_sequences"`
	TopP          float64  `json:"top_p"`
	Temperature   float64  `json:"temperature"`
	MaxTokens     int      `json:"max_tokens"`

	OutputSchema *schema.JSON `json:"output_schema"`

	Tools      []tools.Tool `json:"tools"`
	ToolConfig *tools.Tool  `json:"tool"`
}

type FullRequest struct {
	Request
	Prompts []prompt.Prompt `json:"prompts"`
}
