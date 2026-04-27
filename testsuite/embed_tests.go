package testsuite

import (
	"context"

	"github.com/modfin/bellman/models/embed"
)

var embedManyTexts = []string{
	"The quick brown fox jumps over the lazy dog.",
	"Pack my box with five dozen liquor jugs.",
	"Sphinx of black quartz, judge my vow.",
}

func testEmbedSingle(e embed.Embeder, m embed.Model) func(tester) {
	return func(t tester) {
		res, err := e.Embed(embed.NewSingleRequest(context.Background(), m, "text"))
		if err != nil {
			t.Fatalf("Embed() error = %v", err)
		}

		vec, err := res.Single()
		if err != nil {
			t.Fatalf("Single() error = %v", err)
		}

		if m.OutputDimensions != 0 && len(vec) != m.OutputDimensions {
			t.Fatalf("expected %d dimensions, got %d", m.OutputDimensions, len(vec))
		}
		if len(vec) == 0 {
			t.Fatalf("expected non-empty embedding")
		}
	}
}

func testEmbedMany(e embed.Embeder, m embed.Model) func(tester) {
	return func(t tester) {
		res, err := e.Embed(embed.NewManyRequest(context.Background(), m, embedManyTexts))
		if err != nil {
			t.Fatalf("Embed() error = %v", err)
		}

		if len(res.Embeddings) != len(embedManyTexts) {
			t.Fatalf("expected %d embeddings, got %d", len(embedManyTexts), len(res.Embeddings))
		}
		for i, vec := range res.Embeddings {
			if len(vec) == 0 {
				t.Fatalf("embedding %d is empty", i)
			}
			if m.OutputDimensions != 0 && len(vec) != m.OutputDimensions {
				t.Fatalf("embedding %d: expected %d dimensions, got %d", i, m.OutputDimensions, len(vec))
			}
		}
	}
}

func testEmbedDocument(e embed.Embeder, m embed.Model) func(tester) {
	return func(t tester) {
		res, err := e.EmbedDocument(embed.NewDocumentRequest(context.Background(), m, embedManyTexts))
		if err != nil {
			t.Fatalf("EmbedDocument() error = %v", err)
		}

		if len(res.Embeddings) != len(embedManyTexts) {
			t.Fatalf("expected %d embeddings, got %d", len(embedManyTexts), len(res.Embeddings))
		}
		for i, vec := range res.Embeddings {
			if len(vec) == 0 {
				t.Fatalf("embedding %d is empty", i)
			}
			if m.OutputDimensions != 0 && len(vec) != m.OutputDimensions {
				t.Fatalf("embedding %d: expected %d dimensions, got %d", i, m.OutputDimensions, len(vec))
			}
		}
	}
}
