package gen

import (
	"errors"
	"github.com/modfin/bellman/prompt"
	"strings"
)

type Prompter interface {
	SetRequest(request Request)
	Prompt(prompts ...prompt.Prompt) (*Response, error)
	Stream(prompts ...prompt.Prompt) (<-chan *StreamResponse, error)
}
type Gen interface {
	Provider() string
	Generator(options ...Option) *Generator
}

type Model struct {
	Provider string `json:"provider"`
	Name     string `json:"name"`

	Config map[string]any `json:"config,omitempty"`

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

func ToModel(fqn string) (Model, error) {
	provider, name, found := strings.Cut(fqn, "/")
	if !found {
		return Model{}, errors.New("invalid fqn, did not find a '/' separating provider and model")
	}
	return Model{
		Provider: provider,
		Name:     name,
	}, nil
}
