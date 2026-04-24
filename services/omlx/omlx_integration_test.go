package omlx_test

import (
	"os"
	"testing"

	"github.com/modfin/bellman/models/gen"
	"github.com/modfin/bellman/services/omlx"
	"github.com/modfin/bellman/testsuite"
)

func TestOmlxIntegration(t *testing.T) {
	url := os.Getenv("OMLX_URL")
	key := os.Getenv("OMLX_API_KEY")
	if url == "" || key == "" {
		t.Skip("OMLX_API_KEY/OMLX_URL not set")
	}

	client := omlx.New(url, key)
	g := client.Generator(gen.WithModel(omlx.GenModel_gemma4_26b_a4b_it_5bit))

	testsuite.Run(t, g, testsuite.Capabilities{
		Tools:               true,
		StructuredOutput:    true,
		Streaming:           true,
		Agent:               true,
		StreamThinkingTools: true,
	})
}
