package vertexai

import (
	"github.com/modfin/bellman/tools"
)

type response struct {
	llm   geminiResponse
	tools []tools.Tool
}

type functionCall struct {
	Name string `json:"name"`
	Arg  any    `json:"args"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Role  string `json:"role"`
			Parts []struct {
				Text         string       `json:"text"`
				FunctionCall functionCall `json:"functionCall"`
			} `json:"parts"`
		} `json:"content"`
		FinishReason  string `json:"finishReason"`
		SafetyRatings []struct {
			Category         string  `json:"category"`
			Probability      string  `json:"probability"`
			ProbabilityScore float64 `json:"probabilityScore"`
			Severity         string  `json:"severity"`
			SeverityScore    float64 `json:"severityScore"`
		} `json:"safetyRatings"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
}
