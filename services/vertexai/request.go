package vertexai

import "github.com/modfin/bellman/tools"

type genRequestContent struct {
	Role  string                  `json:"role,omitempty"`
	Parts []genRequestContentPart `json:"parts,omitempty"`
}
type genRequestContentPart struct {
	Text string `json:"text,omitempty"`

	InlineData       *inlineData       `json:"inlineData,omitempty"`
	FileData         *fileData         `json:"fileData,omitempty"`
	FunctionCall     *functionCall     `json:"functionCall,omitempty"`
	FunctionResponse *functionResponse `json:"functionResponse,omitempty"`
}

type inlineData struct {
	MimeType string `json:"mimeType,omitempty"`
	Data     string `json:"data,omitempty"` // base64 encoded. Max 20mb
}
type fileData struct {
	MimeType string `json:"mimeType,omitempty"`
	FileUri  string `json:"fileUri,omitempty"` // uri, eg gs://bucket/file
}

type functionCall struct {
	Name string         `json:"name,omitempty"`
	Args map[string]any `json:"args,omitempty"`
}

type functionResponse struct {
	Name     string `json:"name,omitempty"`
	Response struct {
		Name    string `json:"name,omitempty"`
		Content any    `json:"content,omitempty"`
	} `json:"response,omitempty"`
}

type thinkingConfig struct {
	ThinkingBudget  *int  `json:"thinkingBudget,omitempty"` // -1 for dynamic
	IncludeThoughts *bool `json:"includeThoughts,omitempty"`
}

type genConfig struct {
	MaxOutputTokens *int `json:"maxOutputTokens,omitempty"`

	ThinkingConfig *thinkingConfig `json:"thinkingConfig,omitempty"`

	TopP             *float64 `json:"topP,omitempty"`
	TopK             *int     `json:"topK,omitempty"`
	Temperature      *float64 `json:"temperature,omitempty"`
	FrequencyPenalty *float64 `json:"frequencyPenalty,omitempty"`
	PresencePenalty  *float64 `json:"presencePenalty,omitempty"`

	StopSequences []string `json:"stopSequences,omitempty"`

	ResponseMimeType   *string  `json:"responseMimeType,omitempty"`
	ResponseModalities []string `json:"responseModalities"` // TEXT, AUDIO, IMAGE

	ResponseSchema *JSONSchema `json:"responseSchema,omitempty"`
}

type genTool struct {
	FunctionDeclaration []genToolFunc `json:"functionDeclarations"`
}

type genToolFunc struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  *JSONSchema `json:"parameters"`
}

type genToolConfig struct {
	GoogleFunctionCallingConfig genFunctionCallingConfig `json:"functionCallingConfig"`
}

type genFunctionCallingConfig struct {
	Mode                 string   `json:"mode"`
	AllowedFunctionNames []string `json:"allowedFunctionNames,omitempty"`
}

type genRequest struct {
	Contents          []genRequestContent `json:"contents"`
	SystemInstruction *genRequestContent  `json:"systemInstruction,omitempty"`
	GenerationConfig  *genConfig          `json:"generationConfig,omitempty"`

	Tools      []genTool      `json:"tools,omitempty"`
	ToolConfig *genToolConfig `json:"toolConfig,omitempty"`

	toolBelt map[string]*tools.Tool `json:"-"`
	url      string                 `json:"-"`
}
