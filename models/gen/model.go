package gen

import (
	"github.com/modfin/bellman/prompt"
)

type Prompter interface {
	SetRequest(request Request)
	Prompt(prompts ...prompt.Prompt) (*Response, error)
}
type Gen interface {
	Generator(options ...Option) *Generator
}

type Model struct {
	Provider    string `json:"provider"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`

	InputContentTypes []string `json:"input_content_types,omitempty"`

	InputMaxToken  int `json:"input_max_token,omitempty"`
	OutputMaxToken int `json:"output_max_token,omitempty"`

	SupportTools            bool `json:"support_tools,omitempty"`
	SupportStructuredOutput bool `json:"support_structured_output,omitempty"`
}

func (m Model) FQN() string {
	return m.String()
}
func (m Model) String() string {
	return m.Provider + "/" + m.Name
}
