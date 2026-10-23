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
