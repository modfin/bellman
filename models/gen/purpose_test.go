package gen

import (
	"testing"

	"github.com/modfin/bellman/prompt"
)

type recordingPrompter struct {
	request Request
}

func (p *recordingPrompter) SetRequest(r Request) { p.request = r }
func (p *recordingPrompter) Prompt(...prompt.Prompt) (*Response, error) {
	return &Response{}, nil
}
func (p *recordingPrompter) Stream(...prompt.Prompt) (<-chan *StreamResponse, error) {
	return nil, nil
}

func TestGeneratorPurpose(t *testing.T) {
	p := &recordingPrompter{}
	g := (&Generator{Prompter: p}).Purpose("report.summarize")
	if _, err := g.Prompt(); err != nil {
		t.Fatalf("prompt: %v", err)
	}
	if p.request.Purpose != "report.summarize" {
		t.Errorf("got purpose %q, want %q", p.request.Purpose, "report.summarize")
	}
	if _, err := g.Purpose("Report Summary").Prompt(); err == nil {
		t.Error("want error for invalid purpose")
	}
}
