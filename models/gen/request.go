package gen

import (
	"context"
	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/schema"
	"github.com/modfin/bellman/tools"
)

type Request struct {
	Ctx context.Context `json:"-"`

	Model Model `json:"model"`

	SystemPrompt  string   `json:"system_prompt,omitempty"`
	StopSequences []string `json:"stop_sequences,omitempty"`

	TopP             *float64 `json:"top_p,omitempty"`
	TopK             *int     `json:"top_k,omitempty"`
	Temperature      *float64 `json:"temperature,omitempty"`
	MaxTokens        *int     `json:"max_tokens,omitempty"`
	FrequencyPenalty *float64 `json:"frequency_penalty,omitempty"`
	PresencePenalty  *float64 `json:"presence_penalty,omitempty"`

	OutputSchema *schema.JSON `json:"output_schema,omitempty"`

	Tools      []tools.Tool `json:"tools,omitempty"`
	ToolConfig *tools.Tool  `json:"tool,omitempty"`
}

type FullRequest struct {
	Request
	Prompts []prompt.Prompt `json:"prompts"`
}
