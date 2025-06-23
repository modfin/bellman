package anthropic

const respone_output_callback_name = "__bellman__result_callback"

type anthropicResponse struct {
	Content []struct {
		Type     string `json:"type"` // text or tool_use
		Text     string `json:"text"`
		Thinking string `json:"thinking"`
		Name     string `json:"name"`
		ID       string `json:"id"`
		Input    any    `json:"input"`
	} `json:"content"`
	ID           string `json:"id"`
	Model        string `json:"model"`
	Role         string `json:"role"`
	StopReason   string `json:"stop_reason"`
	StopSequence any    `json:"stop_sequence"`
	Type         string `json:"type"`
	Usage        struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

type anthropicStreamResponse struct {
	Type  string `json:"type"`  // message_start, content_block_start, content_block_delta, content_block_stop, message_delta, message_stop
	Index int    `json:"index"` // Index of the message in the stream

	Message      *anthropicResponse           `json:"message,omitempty"`
	Delta        *anthropicStreamContentBlock `json:"delta,omitempty"`         // Only for content_block_delta and message_delta
	ContentBlock *anthropicStreamContentBlock `json:"content_block,omitempty"` // Only for content_block_delta and message_delta

	Usage *struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type anthropicStreamContentBlock struct {
	ID           *string `json:"id"`
	Name         *string `json:"name,omitempty"`
	Type         string  `json:"type"` // text_delta, input_json_delta, tool_use, text
	Text         *string `json:"text,omitempty"`
	Thinking     *string `json:"thinking,omitempty"`
	PartialJSON  *string `json:"partial_json,omitempty"`
	StopReason   *string `json:"stop_reason,omitempty"`
	StopSequence *any    `json:"stop_sequence,omitempty"`
}
