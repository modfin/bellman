package anthropic

import (
	"encoding/base64"
	"encoding/json"
	"github.com/modfin/bellman/schema"
	"io"
)

// https://docs.anthropic.com/en/api/messages
type request struct {
	Model     string `json:"model"`
	MaxTokens int    `json:"max_tokens"`

	Temperature   float64  `json:"temperature,omitempty"`
	TopP          *float64 `json:"top_p,omitempty"`
	StopSequences []string `json:"stop_sequences,omitempty"`

	// System, System.type must be "text"
	System string `json:"system"`

	Tool  *reqToolChoice `json:"tool_choice,omitempty"`
	Tools []reqTool      `json:"tools,omitempty"`

	Messages []reqMessages `json:"messages"`
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
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	InputSchema *schema.JSON `json:"input_schema,omitempty"`
}

type reqContent struct {
	Type   string            `json:"type"` // eg text, image
	Text   string            `json:"text,omitempty"`
	Source *reqContentSource `json:"source,omitempty"`
}

// https://docs.anthropic.com/en/api/messages-examples#vision
type reqContentSource struct {
	Type      string    `json:"type"`           // eg base64
	MediaType string    `json:"media_type"`     //image/jpeg, image/png, image/gif, and image/webp
	Data      io.Reader `json:"data,omitempty"` // base64 encoded.
}

func (i reqContentSource) MarshalJSON() ([]byte, error) {
	_type, err := json.Marshal(i.Type)
	if err != nil {
		return nil, err
	}

	mime, err := json.Marshal(i.MediaType)
	if err != nil {
		return nil, err
	}
	d, err := io.ReadAll(i.Data)
	if err != nil {
		return nil, err
	}
	return []byte(`{"type":` + string(_type) + `,"media_type":` + string(mime) + `,"data":"` + base64.StdEncoding.EncodeToString(d) + `"}`), nil
}
