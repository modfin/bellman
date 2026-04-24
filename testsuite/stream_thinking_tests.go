package testsuite

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modfin/bellman/models"
	"github.com/modfin/bellman/models/gen"
	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/tools"
)

// testStreamThinkingTools exercises the real consumer pattern: a single
// Stream() call that interleaves text deltas, thinking deltas, and tool-call
// argument deltas, and that also emits per-chunk metadata. The test accumulates
// events the same way a production broker would and asserts that the stitched
// tool-call argument is valid JSON — the one assertion that catches regressions
// across all provider-specific delta emission strategies.
func testStreamThinkingTools(g *gen.Generator) func(*testing.T) {
	return func(t *testing.T) {
		type Args struct {
			City string `json:"city" json-description:"the city to fetch the weather for"`
		}

		weatherTool := tools.NewTool("get_weather",
			tools.WithDescription("fetches the current weather for a given city"),
			tools.WithArgSchema(Args{}),
			tools.WithFunction(func(ctx context.Context, call tools.Call) (string, error) {
				return `{"temp_c":22}`, nil
			}),
		)

		sg := g.
			System("You are a helpful assistant. You MUST call the get_weather tool exactly once to answer any weather question. Think briefly before calling the tool.").
			SetTools(weatherTool).
			IncludeThinkingParts(true)

		stream, err := sg.Stream(prompt.AsUser("Think briefly about which city is warmer this time of year — Stockholm or Madrid — then call get_weather for the warmer one."))
		if err != nil {
			t.Fatalf("Stream() error = %v", err)
		}

		textByIndex := map[int]*strings.Builder{}
		calls := map[string]*tools.Call{}
		var thinkingBuf strings.Builder
		var metadata models.Metadata
		textDeltas := 0
		thinkingDeltas := 0

		for r := range stream {
			switch r.Type {
			case gen.TYPE_DELTA:
				switch r.Role {
				case prompt.AssistantRole:
					textDeltas++
					b, ok := textByIndex[r.Index]
					if !ok {
						b = &strings.Builder{}
						textByIndex[r.Index] = b
					}
					b.WriteString(r.Content)

				case prompt.ToolCallRole:
					if r.ToolCall == nil {
						t.Fatalf("tool-call delta had nil ToolCall")
					}
					c, ok := calls[r.ToolCall.ID]
					if !ok {
						c = &tools.Call{
							ID:   r.ToolCall.ID,
							Name: r.ToolCall.Name,
							Ref:  r.ToolCall.Ref,
						}
						calls[r.ToolCall.ID] = c
					}
					c.Argument = append(c.Argument, r.ToolCall.Argument...)
				}

			case gen.TYPE_THINKING_DELTA:
				thinkingDeltas++
				thinkingBuf.WriteString(r.Content)

			case gen.TYPE_METADATA:
				if r.Metadata != nil {
					metadata.InputTokens += r.Metadata.InputTokens
					metadata.OutputTokens += r.Metadata.OutputTokens
					metadata.ThinkingTokens += r.Metadata.ThinkingTokens
					metadata.TotalTokens += r.Metadata.TotalTokens
					if r.Metadata.Model != "" {
						metadata.Model = r.Metadata.Model
					}
				}

			case gen.TYPE_ERROR:
				t.Fatalf("stream error = %v", r.Content)
			}
		}

		if len(calls) == 0 {
			t.Fatalf("expected at least one tool call in the stream, got none (text deltas=%d, thinking deltas=%d)", textDeltas, thinkingDeltas)
		}

		for id, c := range calls {
			if c.Name != "get_weather" {
				t.Fatalf("expected tool name get_weather, got %q (id=%s)", c.Name, id)
			}
			var a Args
			if err := json.Unmarshal(c.Argument, &a); err != nil {
				t.Fatalf("accumulated tool arg not valid JSON for id=%s: %v (raw=%q)", id, err, string(c.Argument))
			}
			city := strings.ToLower(a.City)
			if !strings.Contains(city, "stockholm") && !strings.Contains(city, "madrid") {
				t.Fatalf("expected city to be Stockholm or Madrid, got %q", a.City)
			}
		}

		if metadata.OutputTokens == 0 && metadata.TotalTokens == 0 {
			t.Fatalf("expected non-zero output/total tokens in metadata, got %+v", metadata)
		}
		if metadata.Model == "" {
			t.Fatalf("expected model name in metadata")
		}
	}
}
