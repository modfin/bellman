package ollama

type genResponseMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type genResponse struct {
	Model     string             `json:"model"`
	CreatedAt string             `json:"created_at"`
	Message   genResponseMessage `json:"message"`

	DoneReason         string `json:"done_reason"`
	Done               bool   `json:"done"`
	TotalDuration      int    `json:"total_duration"`
	LoadDuration       int    `json:"load_duration"`
	PromptEvalCount    int    `json:"prompt_eval_count"`
	PromptEvalDuration int    `json:"prompt_eval_duration"`
	EvalCount          int    `json:"eval_count"`
	EvalDuration       int    `json:"eval_duration"`
}
