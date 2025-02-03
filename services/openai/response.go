package openai

import (
	"github.com/modfin/bellman/tools"
)

type response struct {
	tools []tools.Tool
	llm   openaiResponse
}

type openaiResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int    `json:"created"`
	Model   string `json:"model"`
	Usage   struct {
		PromptTokens            int `json:"prompt_tokens"`
		CompletionTokens        int `json:"completion_tokens"`
		TotalTokens             int `json:"total_tokens"`
		CompletionTokensDetails struct {
			ReasoningTokens          int `json:"reasoning_tokens"`
			AcceptedPredictionTokens int `json:"accepted_prediction_tokens"`
			RejectedPredictionTokens int `json:"rejected_prediction_tokens"`
		} `json:"completion_tokens_details"`
	} `json:"usage"`
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
