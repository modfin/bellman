package vllm

type vllmStreamResponse struct {
	ID                string       `json:"id"`
	Object            string       `json:"object"`
	Created           int          `json:"created"`
	Model             string       `json:"model"`
	ServiceTier       *ServiceTier `json:"service_tier,omitempty"`
	SystemFingerprint string       `json:"system_fingerprint"`
	Choices           []struct {
		Index int `json:"index"`
		Delta struct {
			Content   *string     `json:"content"`
			Refusal   interface{} `json:"refusal"`
			Role      string      `json:"role"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Function struct {
					Arguments string `json:"arguments"`
					Name      string `json:"name"`
				} `json:"function,omitempty"`
				Type string `json:"type"`
			} `json:"tool_calls,omitempty"`
		} `json:"delta"`
		Logprobs     interface{} `json:"logprobs"`
		FinishReason interface{} `json:"finish_reason"`
	} `json:"choices"`
	Usage *usage `json:"usage"`
}

type usage struct {
	PromptTokens        int `json:"prompt_tokens"`
	CompletionTokens    int `json:"completion_tokens"`
	TotalTokens         int `json:"total_tokens"`
	PromptTokensDetails struct {
		CachedTokens int `json:"cached_tokens"`
		AudioTokens  int `json:"audio_tokens"`
	} `json:"prompt_tokens_details"`
	CompletionTokensDetails struct {
		ReasoningTokens          int `json:"reasoning_tokens"`
		AudioTokens              int `json:"audio_tokens"`
		AcceptedPredictionTokens int `json:"accepted_prediction_tokens"`
		RejectedPredictionTokens int `json:"rejected_prediction_tokens"`
	} `json:"completion_tokens_details"`
}

type vllmResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int    `json:"created"`
	Model   string `json:"model"`
	Usage   usage  `json:"usage"`
	Choices []struct {
		Message struct {
			Role      string             `json:"role"`
			Content   string             `json:"content"`
			ToolCalls []responseToolCall `json:"tool_calls"`
		} `json:"message"`
		Logprobs     any    `json:"logprobs"`
		FinishReason string `json:"finish_reason"`
		Index        int    `json:"index"`
	} `json:"choices"`
	ServiceTier *ServiceTier `json:"service_tier,omitempty"`
}

type responseToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Arguments string `json:"arguments"`
		Name      string `json:"name"`
	} `json:"function"`
}

type toolFunc struct {
	Name        string      `json:"name"`
	Parameters  *JSONSchema `json:"parameters,omitempty"`
	Description string      `json:"description,omitempty"`
	Strict      bool        `json:"strict,omitempty"`
}
