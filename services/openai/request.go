package openai

import (
	"github.com/modfin/bellman/tools"
)

// https://platform.openai.com/docs/api-reference/responses

type inputItem interface {
	isInputItem()
}

type messageItem struct {
	Role    string           `json:"role"`
	Content []messageContent `json:"content"`
}

func (messageItem) isInputItem() {}

type messageContent struct {
	Type     string  `json:"type"` // "input_text" | "output_text" | "input_image"
	Text     *string `json:"text,omitempty"`
	ImageURL *string `json:"image_url,omitempty"`
	Detail   *string `json:"detail,omitempty"`
}

type functionCallItem struct {
	Type      string `json:"type"` // "function_call"
	CallID    string `json:"call_id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

func (functionCallItem) isInputItem() {}

type functionCallOutputItem struct {
	Type   string `json:"type"` // "function_call_output"
	CallID string `json:"call_id"`
	Output string `json:"output"`
}

func (functionCallOutputItem) isInputItem() {}

// ReasoningEffort is a string that can be "minimal", "low", "medium", "high", or "none" (model-dependent).
type ReasoningEffort string

const (
	ReasoningEffortNone    ReasoningEffort = "none"
	ReasoningEffortMinimal ReasoningEffort = "minimal"
	ReasoningEffortLow     ReasoningEffort = "low"
	ReasoningEffortMedium  ReasoningEffort = "medium"
	ReasoningEffortHigh    ReasoningEffort = "high"
)

type reasoningConfig struct {
	Effort  *ReasoningEffort `json:"effort,omitempty"`
	Summary *string          `json:"summary,omitempty"`
}

type ServiceTier string

const (
	ServiceTierAuto     ServiceTier = "auto"
	ServiceTierDefault  ServiceTier = "default"
	ServiceTierFlex     ServiceTier = "flex"
	ServiceTierPriority ServiceTier = "priority"
)

type responsesTool struct {
	Type        string      `json:"type"` // always "function"
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Parameters  *JSONSchema `json:"parameters,omitempty"`
	Strict      bool        `json:"strict,omitempty"`
}

type responseTextFormat struct {
	Type   string      `json:"type"` // "json_schema" or "text"
	Name   string      `json:"name,omitempty"`
	Schema *JSONSchema `json:"schema,omitempty"`
	Strict bool        `json:"strict,omitempty"`
}

type textConfig struct {
	Format *responseTextFormat `json:"format,omitempty"`
}

type genRequest struct {
	Model        string      `json:"model"`
	Input        []inputItem `json:"input"`
	Instructions *string     `json:"instructions,omitempty"`

	Tools      []responsesTool `json:"tools,omitempty"`
	ToolChoice any             `json:"tool_choice,omitempty"`

	Text      *textConfig      `json:"text,omitempty"`
	Reasoning *reasoningConfig `json:"reasoning,omitempty"`

	MaxOutputTokens *int     `json:"max_output_tokens,omitempty"`
	Temperature     *float64 `json:"temperature,omitempty"`
	TopP            *float64 `json:"top_p,omitempty"`

	ServiceTier *ServiceTier `json:"service_tier,omitempty"`
	Stream      bool         `json:"stream,omitempty"`
	Store       *bool        `json:"store,omitempty"`

	toolBelt map[string]*tools.Tool
}
