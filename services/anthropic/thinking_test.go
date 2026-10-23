package anthropic

import (
	"encoding/json"
	"testing"

	"github.com/modfin/bellman/models/gen"
)

func TestToThinking(t *testing.T) {
	parts := true

	tests := []struct {
		name       string
		request    gen.Request
		maxTokens  int
		want       string
		wantEffort string
	}{
		{"unset", gen.Request{Model: GenModel_5_5_opus_latest}, 8192, `null`, ""},
		{"adaptive", gen.Request{Model: GenModel_5_5_opus_latest, ThinkingLevel: gen.ThinkingLevelHigh}, 8192, `{"type":"adaptive"}`, "high"},
		{"parts only", gen.Request{Model: GenModel_5_5_opus_latest, ThinkingParts: &parts}, 8192, `{"type":"adaptive","display":"summarized"}`, ""},
		{"minimal rounds up", gen.Request{Model: GenModel_5_1_fable_latest, ThinkingLevel: gen.ThinkingLevelMinimal}, 8192, `{"type":"adaptive"}`, "low"},
		{"none on always thinking model", gen.Request{Model: GenModel_5_5_opus_latest, ThinkingLevel: gen.ThinkingLevelNone}, 8192, `{"type":"adaptive"}`, "low"},
		{"none disables", gen.Request{Model: GenModel_4_8_opus_latest, ThinkingLevel: gen.ThinkingLevelNone}, 8192, `{"type":"disabled"}`, ""},
		{"none on haiku 5.5", gen.Request{Model: GenModel_5_5_haiku_latest, ThinkingLevel: gen.ThinkingLevelNone}, 8192, `{"type":"disabled"}`, ""},
		{"none on sonnet 5.5", gen.Request{Model: GenModel_5_5_sonnet_latest, ThinkingLevel: gen.ThinkingLevelNone, ThinkingParts: &parts}, 8192, `{"type":"between_tools"}`, ""},
		{"no xhigh on 4.6 by name", gen.Request{Model: gen.Model{Provider: Provider, Name: GenModel_4_6_opus_latest.Name}, ThinkingLevel: gen.ThinkingLevelXHigh}, 8192, `{"type":"adaptive"}`, "high"},
		{"no xhigh on 4.6", gen.Request{Model: GenModel_4_6_sonnet_latest, ThinkingLevel: gen.ThinkingLevelXHigh}, 8192, `{"type":"adaptive"}`, "high"},
		{"legacy budget", gen.Request{Model: GenModel_4_5_haiku_latest, ThinkingLevel: gen.ThinkingLevelLow}, 8192, `{"budget_tokens":2048,"type":"enabled"}`, ""},
		{"legacy budget capped by max tokens", gen.Request{Model: GenModel_4_5_sonnet_latest, ThinkingLevel: gen.ThinkingLevelMax}, 5000, `{"budget_tokens":2500,"type":"enabled"}`, ""},
		{"legacy max tokens too low", gen.Request{Model: GenModel_4_0_sonnet_20250514, ThinkingLevel: gen.ThinkingLevelLow}, 2000, `null`, ""},
		{"legacy none", gen.Request{Model: GenModel_3_7_sonnet_latest, ThinkingLevel: gen.ThinkingLevelNone}, 8192, `{"type":"disabled"}`, ""},
		{"legacy budget by name", gen.Request{Model: gen.Model{Provider: Provider, Name: GenModel_4_0_sonnet_20250514.Name}, ThinkingLevel: gen.ThinkingLevelLow}, 8192, `{"budget_tokens":2048,"type":"enabled"}`, ""},
		{"no thinking", gen.Request{Model: GenModel_3_5_haiku_latest, ThinkingLevel: gen.ThinkingLevelHigh, ThinkingParts: &parts}, 8192, `null`, ""},
		{"no thinking by name", gen.Request{Model: gen.Model{Provider: Provider, Name: GenModel_3_5_sonnet_20240620.Name}, ThinkingParts: &parts}, 8192, `null`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			thinking, effort := toThinking(tt.request, tt.maxTokens)
			got, err := json.Marshal(thinking)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if string(got) != tt.want || effort != tt.wantEffort {
				t.Errorf("got %s effort %q, want %s effort %q", got, effort, tt.want, tt.wantEffort)
			}
		})
	}
}
