package vertexai

import (
	"encoding/json"
	"testing"

	"github.com/modfin/bellman/models/gen"
)

func TestToThinkingConfig(t *testing.T) {
	parts := true
	unknown := gen.Model{Name: "gemini-unknown"}

	tests := []struct {
		name    string
		request gen.Request
		want    string
	}{
		{"unset", gen.Request{Model: GenModel_gemini_4_argon_latest}, `null`},
		{"parts only", gen.Request{Model: GenModel_gemini_4_argon_latest, ThinkingParts: &parts}, `{"includeThoughts":true}`},
		{"level", gen.Request{Model: GenModel_gemini_4_argon_latest, ThinkingLevel: gen.ThinkingLevelMedium, ThinkingParts: &parts}, `{"thinkingLevel":"medium","includeThoughts":true}`},
		{"none rounds up", gen.Request{Model: GenModel_gemini_4_argon_latest, ThinkingLevel: gen.ThinkingLevelNone}, `{"thinkingLevel":"low"}`},
		{"max rounds down", gen.Request{Model: GenModel_gemini_4_argon_latest, ThinkingLevel: gen.ThinkingLevelMax}, `{"thinkingLevel":"high"}`},
		{"minimal", gen.Request{Model: GenModel_gemini_3_6_flash_latest, ThinkingLevel: gen.ThinkingLevelNone}, `{"thinkingLevel":"minimal"}`},
		{"name only uses registry", gen.Request{Model: gen.Model{Provider: Provider, Name: GenModel_gemini_3_6_flash_latest.Name}, ThinkingLevel: gen.ThinkingLevelMinimal}, `{"thinkingLevel":"minimal"}`},
		{"unknown model skips minimal", gen.Request{Model: unknown, ThinkingLevel: gen.ThinkingLevelMinimal}, `{"thinkingLevel":"low"}`},
		{"model levels", gen.Request{Model: GenModel_gemini_3_pro_preview, ThinkingLevel: gen.ThinkingLevelMedium}, `{"thinkingLevel":"low"}`},
		{"gemini 2 budget", gen.Request{Model: GenModel_gemini_2_5_flash_latest, ThinkingLevel: gen.ThinkingLevelMedium}, `{"thinkingBudget":8192}`},
		{"gemini 2 budget by name", gen.Request{Model: gen.Model{Provider: Provider, Name: GenModel_gemini_2_5_flash_lite_latest.Name}, ThinkingLevel: gen.ThinkingLevelLow}, `{"thinkingBudget":1024}`},
		{"unknown gemini 2 uses budget", gen.Request{Model: gen.Model{Provider: Provider, Name: "gemini-2.5-flash-preview-09-2025"}, ThinkingLevel: gen.ThinkingLevelMedium}, `{"thinkingBudget":8192}`},
		{"gemini 2 none", gen.Request{Model: GenModel_gemini_2_5_flash_latest, ThinkingLevel: gen.ThinkingLevelNone}, `{"thinkingBudget":0}`},
		{"gemini 2.5 pro can't disable", gen.Request{Model: GenModel_gemini_2_5_pro_latest, ThinkingLevel: gen.ThinkingLevelNone}, `{"thinkingBudget":512}`},
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
