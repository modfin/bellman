package gen

import (
	"context"
	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/schema"
	"github.com/modfin/bellman/tools"
)

type Request struct {
	Context context.Context `json:"-"`

	Stream bool `json:"stream"`

	Model        Model  `json:"model"`
	SystemPrompt string `json:"system_prompt,omitempty"`

	OutputSchema *schema.JSON `json:"output_schema,omitempty"`
	StrictOutput bool         `json:"output_strict,omitempty"`

	Tools         []tools.Tool         `json:"tools,omitempty"`
	ToolConfig    *tools.Tool          `json:"tool,omitempty"`
	WebSearchTool *tools.WebSearchTool `json:"web_search_tool,omitempty"`

	ThinkingBudget *int  `json:"thinking_budget,omitempty"`
	ThinkingParts  *bool `json:"thinking_parts,omitempty"`

	TopP             *float64 `json:"top_p,omitempty"`
	TopK             *int     `json:"top_k,omitempty"`
	Temperature      *float64 `json:"temperature,omitempty"`
	MaxTokens        *int     `json:"max_tokens,omitempty"`
	FrequencyPenalty *float64 `json:"frequency_penalty,omitempty"`
	PresencePenalty  *float64 `json:"presence_penalty,omitempty"`
	StopSequences    []string `json:"stop_sequences,omitempty"`
}

type FullRequest struct {
	Request
	Prompts []prompt.Prompt `json:"prompts"`
}
