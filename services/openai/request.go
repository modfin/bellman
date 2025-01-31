package openai

import (
	"encoding/json"
)

type genRequestMessage struct {
	// https://platform.openai.com/docs/guides/text-generation?lang=curl&text-generation-quickstart-example=json#building-prompts
	// system,assistant or user
	Role    string              `json:"role"`
	Content []genMessageContent `json:"content"`
}

type genMessageContent struct {
	Type     string    `json:"type"` // text or image_url
	Text     string    `json:"text,omitempty"`
	ImageUrl *ImageUrl `json:"image_url,omitempty"`
}

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

type genRequest struct {
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
