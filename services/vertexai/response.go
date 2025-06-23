package vertexai

import (
	"github.com/modfin/bellman/tools"
	"time"
)

type response struct {
	llm   geminiResponse
	tools []tools.Tool
}

type geminiStreamingResponse struct {
	Candidates []struct {
		Content struct {
			Role  string `json:"role"`
			Parts []struct {
				Text         *string       `json:"text"`
				Thought      *bool         `json:"thought,omitempty"`
				FunctionCall *functionCall `json:"functionCall,omitempty"`
			} `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int    `json:"promptTokenCount"`
		CandidatesTokenCount int    `json:"candidatesTokenCount"`
		ThoughtsTokenCount   int    `json:"thoughtsTokenCount"`
		TotalTokenCount      int    `json:"totalTokenCount"`
		TrafficType          string `json:"trafficType"`
		PromptTokensDetails  []struct {
			Modality   string `json:"modality"`
			TokenCount int    `json:"tokenCount"`
		} `json:"promptTokensDetails"`
		CandidatesTokensDetails []struct {
			Modality   string `json:"modality"`
			TokenCount int    `json:"tokenCount"`
		} `json:"candidatesTokensDetails"`
	} `json:"usageMetadata"`
	ModelVersion string    `json:"modelVersion"`
	CreateTime   time.Time `json:"createTime"`
	ResponseID   string    `json:"responseId"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Role  string `json:"role"`
			Parts []struct {
				Text         string       `json:"text"`
				Thought      *bool        `json:"thought,omitempty"`
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
		ThoughtsTokenCount   int `json:"thoughtsTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
}
