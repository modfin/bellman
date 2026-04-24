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
	g := client.Generator(gen.WithModel(vertexai.GenModel_gemini_3_1_flash_lite_preview))

	testsuite.Run(t, g, testsuite.Capabilities{
		Tools:            true,
		StructuredOutput: true,
		Streaming:        true,
		//Agent:            true,// missing thinking signature atm
		StreamThinkingTools: true,
	})

	testsuite.RunEmbed(t, client, vertexai.EmbedModel_text_004, testsuite.EmbedCapabilities{
		Single: true,
		Many:   true,
	})
}
