package voyageai_test

import (
	"os"
	"testing"

	"github.com/modfin/bellman/services/voyageai"
	"github.com/modfin/bellman/testsuite"
)

func TestVoyageAIIntegration(t *testing.T) {
	key := os.Getenv("VOYAGEAI_API_KEY")
	if key == "" {
		t.Skip("VOYAGEAI_API_KEY not set")
	}

	client := voyageai.New(key)

	testsuite.RunEmbed(t, client, voyageai.EmbedModel_voyage_4_lite, testsuite.EmbedCapabilities{
		Single: true,
		Many:   true,
	})
	testsuite.RunEmbed(t, client, voyageai.EmbedModel_voyage_context_3, testsuite.EmbedCapabilities{
		Document: true,
	})
}
