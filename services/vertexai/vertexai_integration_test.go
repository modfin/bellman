package vertexai_test

import (
	"os"
	"testing"

	"github.com/modfin/bellman/models/gen"
	"github.com/modfin/bellman/services/vertexai"
	"github.com/modfin/bellman/testsuite"
)

func TestVertexAIIntegration(t *testing.T) {
	project := os.Getenv("VERTEXAI_PROJECT")
	region := os.Getenv("VERTEXAI_REGION")
	credentials := os.Getenv("VERTEXAI_CREDENTIALS")
	if project == "" || region == "" || credentials == "" {
		t.Skip("VERTEXAI_PROJECT/VERTEXAI_REGION/VERTEXAI_CREDENTIALS not set")
	}

	client, err := vertexai.New(vertexai.GoogleConfig{
		Project:    project,
		Region:     region,
		Credential: credentials,
	})
	if err != nil {
		t.Fatal(err)
	}

	var g *gen.Generator

	g = client.Generator(gen.WithModel(vertexai.GenModel_gemini_3_flash_preview))
	testsuite.Run(t, g, testsuite.Capabilities{
		Tools:               true,
		StructuredOutput:    true,
		Streaming:           true,
		Thinking:            true,
		Agent:               true,
		StreamThinkingTools: true,
		StreamAgentMultiHop: true,
	})
	g = client.Generator(gen.WithModel(vertexai.GenModel_gemini_2_5_flash_latest))
	testsuite.Run(t, g, testsuite.Capabilities{
		Tools:               true,
		StructuredOutput:    true,
		Streaming:           true,
		Thinking:            false,
		Agent:               true,
		StreamThinkingTools: true,
		StreamAgentMultiHop: true,
	})

	testsuite.RunEmbed(t, client, vertexai.EmbedModel_text_004, testsuite.EmbedCapabilities{
		Single: true,
		Many:   true,
	})
}
