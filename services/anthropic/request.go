package anthropic

import "github.com/modfin/bellman/tools"

// https://docs.anthropic.com/en/api/messages
type request struct {
	Stream bool `json:"stream,omitempty"`

	Model     string `json:"model"`
	MaxTokens int    `json:"max_tokens,omitempty"`

	Temperature      *float64 `json:"temperature,omitempty"`
	TopP             *float64 `json:"top_p,omitempty"`
	TopK             *int     `json:"top_k,omitempty"`
	FrequencyPenalty *float64 `json:"frequency_penalty,omitempty"`
	PresencePenalty  *float64 `json:"presence_penalty,omitempty"`

	StopSequences []string `json:"stop_sequences,omitempty"`

	// System, System.type must be "text"
	System string `json:"system,omitempty"`

	Tool  *reqToolChoice `json:"tool_choice,omitempty"`
	Tools []reqTool      `json:"tools,omitempty"`

	Messages []reqMessages `json:"messages"`

	Thinking *reqExtendedThinking `json:"thinking,omitempty"`

	toolBelt map[string]*tools.Tool
}

type reqMessages struct {
	Role    string       `json:"role"` // assistant or user
	Content []reqContent `json:"content"`
}

type reqToolChoice struct {
	// "auto, any, tool"
	Type string `json:"type"`

	// Only for type=tool, name of tool to use.
	Name string `json:"name,omitempty"`
}

type reqTool struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	InputSchema *JSONSchema `json:"input_schema,omitempty"`
}

type reqContent struct {
	Type      string            `json:"type"` // eg text, image
	Text      string            `json:"text,omitempty"`
	Source    *reqContentSource `json:"source,omitempty"`
	ID        string            `json:"id,omitempty"`
	ToolUseID string            `json:"tool_use_id,omitempty"`
	Name      string            `json:"name,omitempty"`
	Input     any               `json:"input,omitempty"`
	Content   any               `json:"content,omitempty"`
}

// https://docs.anthropic.com/en/api/messages-examples#vision
type reqContentSource struct {
	Type      string `json:"type"`           // eg base64
	MediaType string `json:"media_type"`     //image/jpeg, image/png, image/gif, and image/webp
	Data      string `json:"data,omitempty"` // base64 encoded.
}

type ExtendedThinkingType string

const (
	ExtendedThinkingTypeEnabled  ExtendedThinkingType = "enabled"
	ExtendedThinkingTypeDisabled ExtendedThinkingType = "disabled"
)

type reqExtendedThinking struct {
	BudgetTokens int                  `json:"budget_tokens,omitempty"`
	Type         ExtendedThinkingType `json:"type,omitempty"`
}
