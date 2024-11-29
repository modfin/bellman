package vertexai

import "github.com/modfin/bellman/schema"

type genRequestContent struct {
	Role  string                  `json:"role"`
	Parts []genRequestContentPart `json:"parts"`
}
type genRequestContentPart struct {
	Text string `json:"text"`
}

type genConfig struct {
	MaxOutputTokens *int      `json:"maxOutputTokens,omitempty"`
	TopP            *float64  `json:"topP,omitempty"`
	Temperature     *float64  `json:"temperature,omitempty"`
	StopSequences   *[]string `json:"stopSequences"`

	ResponseMimeType *string      `json:"responseMimeType,omitempty"`
	ResponseSchema   *schema.JSON `json:"responseSchema"`
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
	SystemInstruction genRequestContent   `json:"systemInstruction"`
	GenerationConfig  genConfig           `json:"generationConfig"`

	Tools      []genTool      `json:"tools,omitempty"`
	ToolConfig *genToolConfig `json:"toolConfig,omitempty"`
}
