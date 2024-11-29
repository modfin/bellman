package voyageai

import (
	"fmt"
	"testing"
)

func TestVoyageAI_Embed(t *testing.T) {
	// Override the API URL to use the mock server

	model := EmbedModel_voyage_3

	// Create a VoyageAI instance with test config
	config := Config{
		ApiKey:         "",
		EmbeddingModel: model.Name,
	}
	voyageAI, err := New(config)
	if err != nil {
		t.Fatalf("failed to create VoyageAI instance: %v", err)
	}

	// Call the Embed function
	text := "test text"
	embedding, err := voyageAI.Embed(text)
	if err != nil {
		t.Fatalf("Embed() error = %v", err)
	}
	if len(embedding) != model.Dimensions {
		t.Fatalf("Embed() dimension = %v, want %v", len(embedding), model.Dimensions)
	}
	fmt.Println(embedding)
}
