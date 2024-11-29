package bellman

type EmbedModel struct {
	Name        string `json:"name"`
	Description string `json:"description"`

	InputMaxTokens   int `json:"input_max_tokens"`
	OutputDimensions int `json:"output_dimensions"`
}

type GenModel struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`

	InputContentTypes []string `json:"input_content_types,omitempty"`

	InputMaxToken  int `json:"input_max_token,omitempty"`
	OutputMaxToken int `json:"output_max_token,omitempty"`

	SupportTools            bool `json:"support_tools,omitempty"`
	SupportStructuredOutput bool `json:"support_structured_output,omitempty"`
}
