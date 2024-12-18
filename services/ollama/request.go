package ollama

import "github.com/modfin/bellman/schema"

type genRequestMessage struct {
	Role      string   `json:"role"` // system, user, assistant, or tool
	Content   string   `json:"content,omitempty"`
	Images    []string `json:"images,omitempty"`
	ToolCalls []string `json:"tool_calls,omitempty"`
}

// https://github.com/ollama/ollama/blob/main/docs/modelfile.md#valid-parameters-and-values
type genRequestOption struct {
	Temperature *float64 `json:"temperature,omitempty"` // (Default: 0.8)
	TopP        *float64 `json:"top_p,omitempty"`       // (Default: 0.9)
	TopK        *int     `json:"top_k,omitempty"`       // (Default: 40)

	// Maximum number of tokens to predict when generating text. (Default: -1, infinite generation)
	MaxTokens *int `json:"num_predict,omitempty"`

	FrequencyPenalty *float64 `json:"repeat_penalty,omitempty"`
	PresencePenalty  *float64 `json:"presence_penalty,omitempty"`

	StopSequences []string `json:"stop,omitempty"`
}

// / https://github.com/ollama/ollama/blob/main/docs/api.md#generate-a-chat-completion
type genRequest struct {
	Model    string              `json:"model"`
	Messages []genRequestMessage `json:"messages"`

	Format *schema.JSON `json:"format,omitempty"`

	Option genRequestOption `json:"option,omitempty"`
	Stream bool             `json:"stream"`
}
