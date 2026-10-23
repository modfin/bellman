package vertexai

import (
	"encoding/json"
	"testing"

	"github.com/modfin/bellman/models/gen"
)

func TestToThinkingConfig(t *testing.T) {
	parts := true
	argon := gen.Model{Name: "gemini-4-argon"}

	tests := []struct {
		name    string
		request gen.Request
		want    string
	}{
		{"unset", gen.Request{Model: argon}, `null`},
		{"parts only", gen.Request{Model: argon, ThinkingParts: &parts}, `{"includeThoughts":true}`},
		{"level", gen.Request{Model: argon, ThinkingLevel: gen.ThinkingLevelMedium, ThinkingParts: &parts}, `{"thinkingLevel":"medium","includeThoughts":true}`},
		{"none rounds up", gen.Request{Model: argon, ThinkingLevel: gen.ThinkingLevelNone}, `{"thinkingLevel":"minimal"}`},
		{"max rounds down", gen.Request{Model: argon, ThinkingLevel: gen.ThinkingLevelMax}, `{"thinkingLevel":"high"}`},
		{"model levels", gen.Request{Model: GenModel_gemini_3_pro_preview, ThinkingLevel: gen.ThinkingLevelMedium}, `{"thinkingLevel":"low"}`},
		{"gemini 2 budget", gen.Request{Model: GenModel_gemini_2_5_flash_latest, ThinkingLevel: gen.ThinkingLevelMedium}, `{"thinkingBudget":8192}`},
		{"gemini 2 none", gen.Request{Model: GenModel_gemini_2_5_flash_latest, ThinkingLevel: gen.ThinkingLevelNone}, `{"thinkingBudget":0}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(toThinkingConfig(tt.request))
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}
