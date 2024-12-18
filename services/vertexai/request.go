package vertexai

import (
	"github.com/modfin/bellman/schema"
)

type genRequestContent struct {
	Role  string                  `json:"role,omitempty"`
	Parts []genRequestContentPart `json:"parts,omitempty"`
}
type genRequestContentPart struct {
	Text string `json:"text,omitempty"`

	InlineDate *inlineDate `json:"inlineData,omitempty"`
	FileData   *fileDate   `json:"fileData,omitempty"`
}

type inlineDate struct {
	MimeType string `json:"mimeType,omitempty"`
	Data     string `json:"data,omitempty"` // base64 encoded. Max 20mb
}
type fileDate struct {
	MimeType string `json:"mimeType,omitempty"`
	FileUri  string `json:"fileUri,omitempty"` // uri, eg gs://bucket/file
}

type genConfig struct {
	MaxOutputTokens *int `json:"maxOutputTokens,omitempty"`

	TopP             *float64 `json:"topP,omitempty"`
	TopK             *int     `json:"topK,omitempty"`
	Temperature      *float64 `json:"temperature,omitempty"`
	FrequencyPenalty *float64 `json:"frequencyPenalty,omitempty"`
	PresencePenalty  *float64 `json:"presencePenalty,omitempty"`

	StopSequences []string `json:"stopSequences,omitempty"`

	ResponseMimeType   *string  `json:"responseMimeType,omitempty"`
	ResponseModalities []string `json:"responseModalities"` // TEXT, AUDIO, IMAGE

	ResponseSchema *schema.JSON `json:"responseSchema,omitempty"`
}

type genTool struct {
	FunctionDeclaration []genToolFunc `json:"functionDeclarations"`
}

type genToolFunc struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Parameters  *schema.JSON `json:"parameters"`
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
}
