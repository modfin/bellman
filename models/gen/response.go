package gen

import (
	"encoding/json"
	"fmt"
	"github.com/modfin/bellman/models"
	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/tools"
)

type StreamingResponseType string

const TYPE_DELTA StreamingResponseType = "delta"
const TYPE_METADATA StreamingResponseType = "metadata"
const TYPE_EOF StreamingResponseType = "EOF"
const TYPE_ERROR StreamingResponseType = "ERROR"

type StreamResponseError string

func (s StreamResponseError) Error() string {
	return string(s)
}

type StreamResponse struct {
	Type    StreamingResponseType `json:"type"`
	Role    prompt.Role           `json:"role"`
	Index   int                   `json:"index"`
	Content string                `json:"content"`

	Metadata *models.Metadata `json:"metadata,omitempty"`
}

func (r StreamResponse) Error() error {
	if r.Type == TYPE_ERROR {
		return StreamResponseError("streaming response error: " + r.Content)
	}
	return nil
}

type Response struct {
	Texts []string     `json:"texts,omitempty"`
	Tools []tools.Call `json:"tools,omitempty"`

	Metadata models.Metadata `json:"metadata,omitempty"`
}

func (r *Response) Eval() (err error) {
	callbacks, err := r.AsTools()
	if err != nil {
		return err
	}

	count := 0
	for _, tool := range callbacks {

		if tool.Ref == nil {
			return fmt.Errorf("tool %s has no callback", tool.Name)
		}

		if tool.Ref.Function == nil {
			return fmt.Errorf("tool %s has no callback", tool.Name)
		}
		count++
		_, err = tool.Ref.Function(tool.Argument)
		if err != nil {
			return fmt.Errorf("tool %s failed: %w", tool.Name, err)
		}
	}

	if count != len(callbacks) {
		return fmt.Errorf("not all callbacks were evaluated")
	}
	return nil
}

func (r *Response) AsTools() ([]tools.Call, error) {
	if !r.IsTools() {
		return nil, fmt.Errorf("no tool call in response")
	}
	return r.Tools, nil
}

func (r *Response) AsText() (string, error) {
	if !r.IsText() {
		return "", fmt.Errorf("no choices in response")
	}
	return r.Texts[0], nil
}
func (r *Response) Unmarshal(ref any) error {
	text, err := r.AsText()
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(text), ref)
}

func (r *Response) IsText() bool {
	return len(r.Texts) > 0 && len(r.Tools) == 0
}

func (r *Response) IsTools() bool {
	return len(r.Tools) > 0
}
