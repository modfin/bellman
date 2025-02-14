package anthropic

const respone_output_callback_name = "__bellman__result_callback"

type anthropicResponse struct {
	Content []struct {
		Type  string `json:"type"` // text or tool_use
		Text  string `json:"text"`
		Name  string `json:"name"`
		ID    string `json:"id"`
		Input any    `json:"input"`
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
