package testsuite

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/modfin/bellman/agent"
	"github.com/modfin/bellman/models/gen"
	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/tools"
)

func testAgentRun(g *gen.Generator, withThinking bool) func(tester) {
	return func(t tester) {
		type Args struct {
			Symbol string `json:"symbol" json-description:"the ticker symbol of a stock"`
		}
		type Result struct {
			Symbol string  `json:"symbol"`
			Price  float64 `json:"price"`
		}

		var collected []Args

		priceTool := tools.NewTool("get_stock_price",
			tools.WithDescription("get the current price of a stock by its ticker symbol"),
			tools.WithArgSchema(Args{}),
			tools.WithFunction(func(ctx context.Context, call tools.Call) (string, error) {
				var a Args
				if err := json.Unmarshal(call.Argument, &a); err != nil {
					return "", err
				}
				collected = append(collected, a)
				return `{"symbol":"` + a.Symbol + `","price":42.5}`, nil
			}),
		)

		// With thinking enabled, the signature round-trip is a required path:
		// the agent loop must replay the model's signed thinking/reasoning
		// output alongside the tool_use, or the provider rejects the follow-up.
		// Without thinking, the same path runs but doesn't exercise signatures.
		sg := g.
			System("You look up stock prices using the available tool, then return the result as JSON.").
			SetTools(priceTool)
		if withThinking {
			sg = sg.ThinkingBudget(1024).IncludeThinkingParts(true)
		}

		res, err := agent.Run[Result](5, 1, sg,
			prompt.AsUser("What is the price of VOLV-B.ST? Use the tool and then return the symbol and price."),
		)
		if err != nil {
			t.Fatalf("agent.Run() error = %v", err)
		}

		if res.Depth < 1 {
			t.Fatalf("expected at least one tool-call loop, got Depth=%d", res.Depth)
		}
		if len(collected) == 0 {
			t.Fatalf("expected the get_stock_price tool to be called at least once")
		}
		if !strings.EqualFold(collected[0].Symbol, "VOLV-B.ST") {
			t.Fatalf("expected tool call with symbol VOLV-B.ST, got %q", collected[0].Symbol)
		}
		if res.Result.Price <= 0 {
			t.Fatalf("expected non-zero Price in result, got %v", res.Result)
		}
		if res.Metadata.TotalTokens == 0 &&
			res.Metadata.InputTokens == 0 &&
			res.Metadata.OutputTokens == 0 {
			t.Fatalf("expected token accounting in Metadata, got %+v", res.Metadata)
		}
	}
}

func testAgentRunMultiHop(g *gen.Generator, withThinking bool) func(tester) {
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

		sg := g.
			System("You compare stock prices. Call the get_stock_price tool once per symbol, then return the ticker of the cheaper stock as JSON.").
			SetTools(priceTool)
		if withThinking {
			sg = sg.ThinkingBudget(1024).IncludeThinkingParts(true)
		}

		res, err := agent.Run[Result](6, 2, sg,
			prompt.AsUser("Which is cheaper right now — VOLV-B.ST or ERIC-B.ST? Call the tool once per symbol."),
		)
		if err != nil {
			t.Fatalf("agent.Run() error = %v", err)
		}

		if len(collected) < 2 {
			t.Fatalf("expected at least two tool calls (one per symbol), got %d", len(collected))
		}
		if res.Depth < 1 {
			t.Fatalf("expected at least one tool-call loop, got Depth=%d", res.Depth)
		}
		if !strings.EqualFold(res.Result.Cheaper, "ERIC-B.ST") {
			t.Fatalf("expected ERIC-B.ST as the cheaper stock, got %q", res.Result.Cheaper)
		}
	}
}

func formatFloat(f float64) string {
	b, _ := json.Marshal(f)
	return string(b)
}
