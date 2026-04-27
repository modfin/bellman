package anthropic_test

import (
	"os"
	"testing"

	"github.com/modfin/bellman/models/gen"
	"github.com/modfin/bellman/services/anthropic"
	"github.com/modfin/bellman/testsuite"
)

func TestAnthropicIntegration(t *testing.T) {
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		t.Skip("ANTHROPIC_API_KEY not set")
	}

	client := anthropic.New(key)

	var g *gen.Generator

	g = client.Generator(gen.WithModel(anthropic.GenModel_4_5_haiku_latest))
	testsuite.Run(t, g, testsuite.Capabilities{
		Tools:               true,
		StructuredOutput:    true,
		Streaming:           true,
		Thinking:            true,
		Agent:               true,
		StreamThinkingTools: true,
		StreamAgentMultiHop: true,
	})
}
