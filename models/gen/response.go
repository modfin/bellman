package gen

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modfin/bellman/models"
	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/tools"
)

type StreamingResponseType string

const TYPE_DELTA StreamingResponseType = "delta"
const TYPE_THINKING_DELTA StreamingResponseType = "thinking_delta"
const TYPE_BLOCK StreamingResponseType = "block" // a finalized replay-ready prompt (thinking or assistant-text with signature); Role tells which
const TYPE_METADATA StreamingResponseType = "metadata"
const TYPE_EOF StreamingResponseType = "EOF"
const TYPE_ERROR StreamingResponseType = "ERROR"

type StreamResponseError string

func (s StreamResponseError) Error() string {
	return string(s)
}

type StreamResponse struct {
	Type     StreamingResponseType `json:"type"`
	Role     prompt.Role           `json:"role"`
	Index    int                   `json:"index"`
	Content  string                `json:"content"`
	ToolCall *tools.Call           `json:"tool_call,omitempty"` // Only for TYPE_DELTA

	// Block is a finalized replay-ready prompt (the streaming analog of a
	// Response.Turn entry). Only set for TYPE_BLOCK events — Role tells you
	// what kind of block it is (ThinkingRole, AssistantRole, ...).
	Block *prompt.Prompt `json:"block,omitempty"`

	Metadata *models.Metadata `json:"metadata,omitempty"`
}

func (r StreamResponse) Error() error {
	if r.Type == TYPE_ERROR {
		return StreamResponseError("streaming response error: " + r.Content)
	}
	return nil
}

type Response struct {
	Texts    []string     `json:"texts,omitempty"`
	Thinking []string     `json:"thinking,omitempty"` // visible thinking text (back-compat)
	Tools    []tools.Call `json:"tools,omitempty"`

	// Turn is the assistant side of this exchange in replay-ready form:
	// thinking blocks first, then (if present) the final assistant text with
	// any signature, in the order the provider produced them. Append it
	// directly to your prompts slice on the next call to preserve the
	// provider's chain-of-thought signatures.
	Turn []prompt.Prompt `json:"turn,omitempty"`

	Metadata models.Metadata `json:"metadata,omitempty"`
}

func (r *Response) Eval(ctx context.Context) (err error) {
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
		_, err = tool.Ref.Function(ctx, tool)
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
