package testsuite

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/modfin/bellman/models/gen"
	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/schema"
	"github.com/modfin/bellman/tools"
)

func testStreamAgentMultiHop(g *gen.Generator, withThinking bool) func(tester) {
	return func(t tester) {
		type Args struct {
			Symbol string `json:"symbol" json-description:"the ticker symbol of a stock"`
		}
		type Result struct {
			Cheaper string `json:"cheaper" json-description:"the ticker symbol of the cheaper stock"`
		}

		var collected []Args
		prices := map[string]float64{
			"VOLV-B.ST": 250.0,
			"ERIC-B.ST": 80.0,
		}

		priceTool := tools.NewTool("get_stock_price",
			tools.WithDescription("get the current price of a stock by its ticker symbol. Only returns one price per call."),
			tools.WithArgSchema(Args{}),
			tools.WithFunction(func(ctx context.Context, call tools.Call) (string, error) {
				var a Args
				if err := json.Unmarshal(call.Argument, &a); err != nil {
					return "", err
				}
				collected = append(collected, a)
				p, ok := prices[strings.ToUpper(a.Symbol)]
				if !ok {
					return `{"error":"unknown symbol"}`, nil
				}
				return `{"symbol":"` + a.Symbol + `","price":` + formatFloat(p) + `}`, nil
			}),
		)

		var result Result
		sg := g.
			System("You compare stock prices. Call the get_stock_price tool once per symbol, then return the ticker of the cheaper stock as JSON.").
			SetTools(priceTool).
			Output(schema.From(result))
		if withThinking {
			sg = sg.ThinkingBudget(1024).IncludeThinkingParts(true)
		}

		prompts := []prompt.Prompt{
			prompt.AsUser("Which is cheaper right now — VOLV-B.ST or ERIC-B.ST? Call the tool once per symbol."),
		}

		var finalText strings.Builder
		toolCallBlocks := 0
		thinkingBlocks := 0

		const maxDepth = 6
		hops := 0
		for depth := 0; depth < maxDepth; depth++ {
			hops++
			stream, err := sg.Stream(prompts...)
			if err != nil {
				t.Fatalf("depth %d: Stream() error = %v", depth, err)
			}

			calls := map[string]*tools.Call{}
			var blocks []prompt.Prompt
			var turnText strings.Builder

			for r := range stream {
				switch r.Type {
				case gen.TYPE_DELTA:
					if r.Role == prompt.AssistantRole {
						turnText.WriteString(r.Content)
					}

				case gen.TYPE_BLOCK:
					if r.Block == nil {
						continue
					}
					blocks = append(blocks, *r.Block)
					switch r.Block.Role {
					case prompt.ThinkingRole:
						thinkingBlocks++
					case prompt.ToolCallRole:
						toolCallBlocks++
						if r.ToolCall == nil {
							t.Fatalf("depth %d: tool-call BLOCK missing sibling ToolCall (Ref must travel on the BLOCK event)", depth)
						}
						if r.ToolCall.Ref == nil || r.ToolCall.Ref.Function == nil {
							t.Fatalf("depth %d: tool-call BLOCK ToolCall.Ref is nil/uncallable for tool %q", depth, r.ToolCall.Name)
						}
						calls[r.ToolCall.ID] = &tools.Call{
							ID:       r.ToolCall.ID,
							Name:     r.ToolCall.Name,
							Argument: r.ToolCall.Argument,
							Ref:      r.ToolCall.Ref,
						}
					}

				case gen.TYPE_ERROR:
					t.Fatalf("depth %d: stream error = %v", depth, r.Content)
				}
			}

			if len(blocks) == 0 {
				t.Fatalf("depth %d: no blocks received from stream (turnText=%q)", depth, turnText.String())
			}

			// Replay the assistant turn verbatim — every replay-ready prompt
			// arrived as a BLOCK with any signature attached to Prompt.Replay.
			prompts = append(prompts, blocks...)

			if len(calls) == 0 {
				// No tool calls — final answer turn.
				finalText.WriteString(turnText.String())
				break
			}

			for _, c := range calls {
				resp, err := c.Ref.Function(context.Background(), *c)
				if err != nil {
					t.Fatalf("depth %d: tool %s failed: %v", depth, c.Name, err)
				}
				prompts = append(prompts, prompt.AsToolResponse(c.ID, c.Name, resp))
			}
		}

		if hops < 2 {
			t.Fatalf("expected at least two hops to exercise replay, got %d", hops)
		}
		if len(collected) < 2 {
			t.Fatalf("expected at least two tool calls (one per symbol), got %d", len(collected))
		}
		if toolCallBlocks < 2 {
			t.Fatalf("expected at least two tool-call BLOCKs across the run, got %d", toolCallBlocks)
		}
		if withThinking && thinkingBlocks == 0 {
			t.Logf("warning: no thinking BLOCKs observed across the run (thinking was requested)")
		}

		var got Result
		if err := json.Unmarshal([]byte(finalText.String()), &got); err != nil {
			t.Fatalf("final text was not valid Result JSON: %v (raw=%q)", err, finalText.String())
		}
		if !strings.EqualFold(got.Cheaper, "ERIC-B.ST") {
			t.Fatalf("expected ERIC-B.ST as cheaper, got %q", got.Cheaper)
		}
	}
}
