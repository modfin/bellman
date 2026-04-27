package xai_test

import (
	"os"
	"testing"

	"github.com/modfin/bellman/models/gen"
	"github.com/modfin/bellman/services/xai"
	"github.com/modfin/bellman/testsuite"
)

func TestXAIIntegration(t *testing.T) {
	key := os.Getenv("XAI_API_KEY")
	if key == "" {
		t.Skip("XAI_API_KEY not set")
	}

	client := xai.New(key)
	g := client.Generator(gen.WithModel(xai.GenModel_grok_4_1_fast_reasoning))

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
