package anthropic

type anthropicResponse struct {
	Content []struct {
		Type      string `json:"type"` // text | thinking | redacted_thinking | tool_use
		Text      string `json:"text"`
		Thinking  string `json:"thinking"`
		Signature string `json:"signature,omitempty"` // on thinking blocks
		Data      string `json:"data,omitempty"`      // on redacted_thinking blocks
		Name      string `json:"name"`
		ID        string `json:"id"`
		Input     any    `json:"input"`
	} `json:"content"`
	ID           string         `json:"id"`
	Model        string         `json:"model"`
	Role         string         `json:"role"`
	StopReason   string         `json:"stop_reason"`
	StopSequence any            `json:"stop_sequence"`
	Type         string         `json:"type"`
	Usage        anthropicUsage `json:"usage"`
	Error        struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// anthropicUsage reports input_tokens exclusive of the cache counters, so the
// full input of a request is input + cache_read + cache_creation.
type anthropicUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
}

// inputTokens is the total input of the request, cached parts included.
func (u anthropicUsage) inputTokens() int {
	return u.InputTokens + u.CacheReadInputTokens + u.CacheCreationInputTokens
}

// mergeMax folds a usage frame into u. A stream reports usage in pieces - the
// input and cache counters on message_start, where output_tokens is only a
// placeholder, and the final output count on message_delta - and each counter
// only grows within a message, so keeping the larger value per field yields the
// running total for the message so far.
func (u *anthropicUsage) mergeMax(o anthropicUsage) {
	u.InputTokens = max(u.InputTokens, o.InputTokens)
	u.OutputTokens = max(u.OutputTokens, o.OutputTokens)
	u.CacheReadInputTokens = max(u.CacheReadInputTokens, o.CacheReadInputTokens)
	u.CacheCreationInputTokens = max(u.CacheCreationInputTokens, o.CacheCreationInputTokens)
}

type anthropicStreamResponse struct {
	Type  string `json:"type"`  // message_start, content_block_start, content_block_delta, content_block_stop, message_delta, message_stop
	Index int    `json:"index"` // Index of the message in the stream

	Message      *anthropicResponse           `json:"message,omitempty"`
	Delta        *anthropicStreamContentBlock `json:"delta,omitempty"`         // Only for content_block_delta and message_delta
	ContentBlock *anthropicStreamContentBlock `json:"content_block,omitempty"` // Only for content_block_delta and message_delta

	Usage *anthropicUsage `json:"usage"`
}

type anthropicStreamContentBlock struct {
	ID           *string `json:"id"`
	Name         *string `json:"name,omitempty"`
	Type         string  `json:"type"` // text_delta, input_json_delta, tool_use, text, thinking_delta, signature_delta, thinking, redacted_thinking
	Text         *string `json:"text,omitempty"`
	Thinking     *string `json:"thinking,omitempty"`
	Signature    *string `json:"signature,omitempty"`
	Data         *string `json:"data,omitempty"` // redacted_thinking
	PartialJSON  *string `json:"partial_json,omitempty"`
	StopReason   *string `json:"stop_reason,omitempty"`
	StopSequence *any    `json:"stop_sequence,omitempty"`
}
