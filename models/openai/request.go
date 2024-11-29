package openai

import (
	"encoding/base64"
	"encoding/json"
	schema "github.com/modfin/bellman/schema"
	"io"
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
	Url    string `json:"url"` /// data:image/jpeg;base64,......
	reader io.Reader
}

func (i ImageUrl) MarshalJSON() ([]byte, error) {
	if len(i.Url) > 0 {
		return json.Marshal(i.Url)
	}
	d, err := io.ReadAll(i.reader)
	if err != nil {
		return nil, err
	}

	return []byte(`{"url": "data:image/jpeg;base64,` + base64.StdEncoding.EncodeToString(d) + `"}`), nil
}

type genRequest struct {
	Model          string              `json:"model"`
	Messages       []genRequestMessage `json:"messages"`
	ResponseFormat *responseFormat     `json:"response_format,omitempty"`

	Tools      []requestTool `json:"tools,omitempty"`
	ToolChoice any           `json:"tool_choice,omitempty"`

	Stop        []string `json:"stop,omitempty"`
	Temperature float64  `json:"temperature"`
	TopP        float64  `json:"top_p"`
	MaxTokens   int      `json:"max_completion_tokens"`
}

type responseFormatSchema struct {
	Name   string       `json:"name"`
	Strict bool         `json:"strict"`
	Schema *schema.JSON `json:"schema"`
}

type responseFormat struct {
	Type string `json:"type"`

	ResponseFormatSchema responseFormatSchema `json:"json_schema"`
}

type requestTool struct {
	Type     string   `json:"type"` // Always function
	Function toolFunc `json:"function"`
}
