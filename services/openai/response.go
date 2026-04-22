package openai

type responseUsage struct {
	InputTokens        int `json:"input_tokens"`
	InputTokensDetails struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"input_tokens_details"`
	OutputTokens        int `json:"output_tokens"`
	OutputTokensDetails struct {
		ReasoningTokens int `json:"reasoning_tokens"`
	} `json:"output_tokens_details"`
	TotalTokens int `json:"total_tokens"`
}

type outputContent struct {
	Type string `json:"type"` // "output_text" | "refusal"
	Text string `json:"text"`
}

type outputReasoningSummary struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type outputItem struct {
	Type      string                   `json:"type"` // "message" | "function_call" | "reasoning"
	ID        string                   `json:"id"`
	Role      string                   `json:"role,omitempty"`    // for "message"
	Content   []outputContent          `json:"content,omitempty"` // for "message"
	CallID    string                   `json:"call_id,omitempty"` // for "function_call"
	Name      string                   `json:"name,omitempty"`
	Arguments string                   `json:"arguments,omitempty"`
	Summary   []outputReasoningSummary `json:"summary,omitempty"` // for "reasoning"
}

type openaiResponseError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type openaiResponseIncomplete struct {
	Reason string `json:"reason,omitempty"`
}

type openaiResponse struct {
	ID                string                    `json:"id"`
	Object            string                    `json:"object"`
	Model             string                    `json:"model"`
	Status            string                    `json:"status"`
	Output            []outputItem              `json:"output"`
	Usage             responseUsage             `json:"usage"`
	ServiceTier       *ServiceTier              `json:"service_tier,omitempty"`
	Error             *openaiResponseError      `json:"error,omitempty"`
	IncompleteDetails *openaiResponseIncomplete `json:"incomplete_details,omitempty"`
}

// streamEventItem is the `item` payload attached to response.output_item.added/done
// (and, via its parent, referenced by function_call_arguments.* events).
type streamEventItem struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	CallID    string `json:"call_id,omitempty"`
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
	Role      string `json:"role,omitempty"`
}

// streamEvent is the union envelope for every SSE event on /v1/responses.
// The `type` field discriminates; unused fields stay zero-valued.
type streamEvent struct {
	Type           string           `json:"type"`
	SequenceNumber int              `json:"sequence_number,omitempty"`
	OutputIndex    int              `json:"output_index,omitempty"`
	ContentIndex   int              `json:"content_index,omitempty"`
	ItemID         string           `json:"item_id,omitempty"`
	Item           *streamEventItem `json:"item,omitempty"`
	Delta          string           `json:"delta,omitempty"`
	Arguments      string           `json:"arguments,omitempty"`
	Text           string           `json:"text,omitempty"`
	Response       *openaiResponse  `json:"response,omitempty"`
	Code           string           `json:"code,omitempty"`
	Message        string           `json:"message,omitempty"`
}
