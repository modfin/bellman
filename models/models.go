package models

type Metadata struct {
	// Cache counts are subsets of InputTokens, reported by the provider.
	// Zero can also mean cache usage was not reported.
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`

	Model          string         `json:"model,omitempty"`
	InputTokens    int            `json:"input_tokens,omitempty"`
	ThinkingTokens int            `json:"thinking_tokens,omitempty"`
	OutputTokens   int            `json:"output_tokens,omitempty"`
	TotalTokens    int            `json:"total_tokens,omitempty"`
	Other          map[string]any `json:"other,omitempty"`
}
