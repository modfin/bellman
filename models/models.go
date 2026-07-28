package models

type Metadata struct {
	Model          string `json:"model,omitempty"`
	InputTokens    int    `json:"input_tokens,omitempty"`
	ThinkingTokens int    `json:"thinking_tokens,omitempty"`
	OutputTokens   int    `json:"output_tokens,omitempty"`
	TotalTokens    int    `json:"total_tokens,omitempty"`

	// CachedTokens is the part of the input that was served from the provider's
	// prompt cache instead of being processed again. It is a subset of the
	// input, not an addition to it, and is billed at a discount.
	CachedTokens int `json:"cached_tokens,omitempty"`

	// CacheWriteTokens is the part of the input that was written to the prompt
	// cache by this request, billed at a premium. Only providers with explicit
	// cache breakpoints report it; providers that cache implicitly leave it 0.
	CacheWriteTokens int `json:"cache_write_tokens,omitempty"`

	Other map[string]any `json:"other,omitempty"`
}
