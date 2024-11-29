package vertexai

import (
	"encoding/base64"
	"encoding/json"
	"github.com/modfin/bellman/schema"
	"io"
)

type genRequestContent struct {
	Role  string                  `json:"role"`
	Parts []genRequestContentPart `json:"parts"`
}
type genRequestContentPart struct {
	Text string `json:"text,omitempty"`

	InlineDate *inlineDate `json:"inlineData,omitempty"`
}

type inlineDate struct {
	MimeType string    `json:"mimeType,omitempty"`
	Data     io.Reader `json:"data,omitempty"` // base64 encoded. Max 20mb
}

func (i inlineDate) MarshalJSON() ([]byte, error) {
	d, err := io.ReadAll(i.Data)
	if err != nil {
		return nil, err
	}
	mime, err := json.Marshal(i.MimeType)
	if err != nil {
		return nil, err
	}
	return []byte(`{"mimeType":` + string(mime) + `,"data":"` + base64.StdEncoding.EncodeToString(d) + `"}`), nil
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
