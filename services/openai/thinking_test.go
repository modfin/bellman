package openai

import (
	"testing"

	"github.com/modfin/bellman/models/gen"
	"github.com/modfin/bellman/prompt"
)

func TestReasoningEffort(t *testing.T) {
	tests := []struct {
		name  string
		model gen.Model
		level gen.ThinkingLevel
		want  ReasoningEffort
	}{
		{"model levels", GenModel_gpt6_luna_latest, gen.ThinkingLevelMax, ReasoningEffortMax},
		{"model without levels", GenModel_gpt5_mini_latest, gen.ThinkingLevelMax, ReasoningEffortHigh},
		{"unknown model", gen.Model{Provider: Provider, Name: "gpt-unknown"}, gen.ThinkingLevelXHigh, ReasoningEffortHigh},
		{"unknown model none", gen.Model{Provider: Provider, Name: "gpt-unknown"}, gen.ThinkingLevelNone, ReasoningEffortNone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &generator{openai: New(""), request: gen.Request{Model: tt.model, ThinkingLevel: tt.level}}
			_, req, err := g.prompt(prompt.AsUser("hi"))
			if err != nil {
				t.Fatalf("prompt: %v", err)
			}
			if req.Reasoning == nil || req.Reasoning.Effort == nil {
				t.Fatalf("no reasoning effort set")
			}
			if *req.Reasoning.Effort != tt.want {
				t.Errorf("got %q, want %q", *req.Reasoning.Effort, tt.want)
			}
		})
	}
}
