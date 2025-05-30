package openai

import (
	"encoding/json"
	"github.com/modfin/bellman/tools"
)

// https://platform.openai.com/docs/api-reference/chat/create

type genRequestMessage interface {
	GetRole() string
}

type genRequestMessageContent struct {
	Type     string    `json:"type"`
	Text     *string   `json:"text,omitempty"`
	ImageUrl *ImageUrl `json:"image_url,omitempty"`
}

type genRequestMessageText struct {
	Role    string                     `json:"role"`
	Content []genRequestMessageContent `json:"content"`
}

func (g genRequestMessageText) GetRole() string { return g.Role }

type genRequestMessageToolCallFunction struct {
	Name      string `json:"name"`
	Arguments any    `json:"arguments"`
}
type genRequestMessageToolCall struct {
	ID       string                            `json:"id"`
	Type     string                            `json:"type"`
	Function genRequestMessageToolCallFunction `json:"function"`
}

type genRequestMessageToolCalls struct {
	Role      string                      `json:"role"`
	ToolCalls []genRequestMessageToolCall `json:"tool_calls"`
}

func (g genRequestMessageToolCalls) GetRole() string { return g.Role }

type genRequestMessageToolResponse struct {
	Role       string `json:"role"`
	Content    any    `json:"content"`
	ToolCallID string `json:"tool_call_id"`
}

func (g genRequestMessageToolResponse) GetRole() string { return g.Role }

type ImageUrl struct {
	Url  string `json:"url"` /// data:image/jpeg;base64,......
	data string
}

func (i ImageUrl) MarshalJSON() ([]byte, error) {
	if len(i.Url) > 0 {
		return json.Marshal(i.Url)
	}
	return []byte(`{"url": "data:image/jpeg;base64,` + i.data + `"}`), nil
}

type StreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type genRequest struct {
	Stream        bool           `json:"stream,omitempty"`
	StreamOptions *StreamOptions `json:"stream_options,omitempty"`

	Model          string              `json:"model"`
	Messages       []genRequestMessage `json:"messages"`
	ResponseFormat *responseFormat     `json:"response_format,omitempty"`

	Tools      []requestTool `json:"tools,omitempty"`
	ToolChoice any           `json:"tool_choice,omitempty"`

	Stop []string `json:"stop,omitempty"`

	MaxTokens        *int     `json:"max_completion_tokens,omitempty"`
	Temperature      *float64 `json:"temperature,omitempty"`
	TopP             *float64 `json:"top_p,omitempty"`
	FrequencyPenalty *float64 `json:"frequency_penalty,omitempty"`
	PresencePenalty  *float64 `json:"presence_penalty,omitempty"`

	toolBelt map[string]*tools.Tool
}

type responseFormatSchema struct {
	Name   string      `json:"name"`
	Strict bool        `json:"strict"`
	Schema *JSONSchema `json:"schema"`
}

type responseFormat struct {
	Type string `json:"type"`

	ResponseFormatSchema responseFormatSchema `json:"json_schema"`
}

type requestTool struct {
	Type     string   `json:"type"` // Always function
	Function toolFunc `json:"function"`
}
