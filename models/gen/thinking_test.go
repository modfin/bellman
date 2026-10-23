package gen

import "testing"

func TestThinkingLevelResolve(t *testing.T) {
	gemini3Pro := []ThinkingLevel{ThinkingLevelLow, ThinkingLevelHigh}

	tests := []struct {
		level     ThinkingLevel
		supported []ThinkingLevel
		want      ThinkingLevel
	}{
		{ThinkingLevelMedium, ThinkingLevels, ThinkingLevelMedium},
		{ThinkingLevelMedium, gemini3Pro, ThinkingLevelLow},
		{ThinkingLevelXHigh, gemini3Pro, ThinkingLevelHigh},
		{ThinkingLevelNone, gemini3Pro, ThinkingLevelLow},
		{ThinkingLevelMax, []ThinkingLevel{ThinkingLevelNone, ThinkingLevelHigh, ThinkingLevelXHigh}, ThinkingLevelXHigh},
		{ThinkingLevelMinimal, []ThinkingLevel{ThinkingLevelNone, ThinkingLevelLow, ThinkingLevelHigh}, ThinkingLevelLow},
		{ThinkingLevelHigh, []ThinkingLevel{ThinkingLevelNone}, ThinkingLevelNone},
		{ThinkingLevelMedium, nil, ThinkingLevelMedium},
		{"", gemini3Pro, ""},
		{"bogus", gemini3Pro, "bogus"},
	}
	for _, tt := range tests {
		if got := tt.level.Resolve(tt.supported); got != tt.want {
			t.Errorf("%q.Resolve(%v) = %q, want %q", tt.level, tt.supported, got, tt.want)
		}
	}
}

func TestModelLookup(t *testing.T) {
	known := Model{Name: "known", ThinkingLevels: []ThinkingLevel{ThinkingLevelHigh}}
	registry := map[string]Model{known.Name: known}

	own := Model{Name: "known", ThinkingLevels: []ThinkingLevel{ThinkingLevelLow}}
	if got := own.Lookup(registry); got.ThinkingLevels[0] != ThinkingLevelLow {
		t.Errorf("own details: got %v", got.ThinkingLevels)
	}
	budget := Model{Name: "known", UsesThinkingBudget: true}
	if got := budget.Lookup(registry); !got.UsesThinkingBudget {
		t.Errorf("own budget: got %+v", got)
	}
	if got := (Model{Name: "known"}).Lookup(registry); got.ThinkingLevels[0] != ThinkingLevelHigh {
		t.Errorf("registry details: got %v", got.ThinkingLevels)
	}
	if got := (Model{Name: "unknown"}).Lookup(registry); got.Name != "unknown" || got.ThinkingLevels != nil {
		t.Errorf("unknown: got %+v", got)
	}
}

func TestCheckThinkingLevel(t *testing.T) {
	for _, level := range []ThinkingLevel{"", ThinkingLevelNone, ThinkingLevelMax} {
		if err := checkThinkingLevel(level); err != nil {
			t.Errorf("%q: unexpected error %v", level, err)
		}
	}
	for _, level := range []ThinkingLevel{"High", "medum"} {
		if err := checkThinkingLevel(level); err == nil {
			t.Errorf("%q: want error", level)
		}
	}
}
