package main

import (
	"testing"

	"github.com/modfin/bellman/models/gen"
)

func TestDecodeGenRequestThinking(t *testing.T) {
	tests := []struct {
		body string
		want gen.ThinkingLevel
	}{
		{`{"prompts":[]}`, ""},
		{`{"thinking_level":"high"}`, gen.ThinkingLevelHigh},
		{`{"thinking_budget":0}`, gen.ThinkingLevelNone},
		{`{"thinking_budget":1024}`, gen.ThinkingLevelLow},
		{`{"thinking_budget":5000}`, gen.ThinkingLevelMedium},
		{`{"thinking_budget":10000}`, gen.ThinkingLevelHigh},
		{`{"thinking_budget":-1}`, ""},
		{`{"thinking_budget":1024,"thinking_level":"max"}`, gen.ThinkingLevelMax},
	}
	for _, tt := range tests {
		req, err := decodeGenRequest([]byte(tt.body))
		if err != nil {
			t.Fatalf("decode %s: %v", tt.body, err)
		}
		if req.ThinkingLevel != tt.want {
			t.Errorf("decode %s: got %q, want %q", tt.body, req.ThinkingLevel, tt.want)
		}
	}
}
