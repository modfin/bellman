package openai_test

import (
	"os"
	"testing"

	"github.com/modfin/bellman/models/gen"
	"github.com/modfin/bellman/services/openai"
	"github.com/modfin/bellman/testsuite"
)

func TestOpenAIIntegration(t *testing.T) {
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		t.Skip("OPENAI_API_KEY not set")
	}

	client := openai.New(key)
	var g *gen.Generator

	g = client.Generator(gen.WithModel(openai.GenModel_gpt5_4_mini_latest))
	testsuite.Run(t, g, testsuite.Capabilities{
		Tools:               true,
		StructuredOutput:    true,
		Streaming:           true,
		Thinking:            true,
		Agent:               true,
		StreamThinkingTools: true,
		StreamAgentMultiHop: true,
	})
	g = client.Generator(gen.WithModel(openai.GenModel_gpt4_1_mini_latest))
	testsuite.Run(t, g, testsuite.Capabilities{
		Tools:               true,
		StructuredOutput:    true,
		Streaming:           true,
		Agent:               true,
		StreamAgentMultiHop: true,
	})

	testsuite.RunEmbed(t, client, openai.EmbedModel_text3_small, testsuite.EmbedCapabilities{
		Single: true,
		Many:   true,
	})
}
