package testsuite

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modfin/bellman/agent"
	"github.com/modfin/bellman/models/gen"
	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/tools"
)

func testAgentRun(g *gen.Generator) func(*testing.T) {
	return func(t *testing.T) {
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

		res, err := agent.Run[Result](5, 1,
			g.System("You look up stock prices using the available tool, then return the result as JSON.").SetTools(priceTool),
			prompt.AsUser("What is the price of VOLVO-B? Use the tool and then return the symbol and price."),
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
		if !strings.EqualFold(collected[0].Symbol, "VOLVO-B") {
			t.Fatalf("expected tool call with symbol VOLVO-B, got %q", collected[0].Symbol)
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
