package vertexai

import (
	"github.com/modfin/bellman/models/embed"
	"github.com/modfin/bellman/models/gen"
)

// https://cloud.google.com/vertex-ai/generative-ai/docs/learn/models#gemini-models

const Provider = "VertexAI"

var GenModel_gemini_2_0_flash_001 = gen.Model{
	Provider:                Provider,
	Name:                    "gemini-2.0-flash-001",
	Description:             "Fast and versatile performance across a diverse variety of tasks",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}

var GenModel_gemini_2_0_flash = gen.Model{
	Provider:                Provider,
	Name:                    "gemini-2.0-flash-exp", // Region: "us-central1",
	Description:             "Fast and versatile performance across a diverse variety of tasks",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}

var GenModel_gemini_1_5_flash = gen.Model{
	Provider:                Provider,
	Name:                    "gemini-1.5-flash",
	Description:             "Fast and versatile performance across a diverse variety of tasks",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}
var GenModel_gemini_1_5_flash_001 = gen.Model{
	Provider:                Provider,
	Name:                    "gemini-1.5-flash-002",
	Description:             "Fast and versatile performance across a diverse variety of tasks",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}
var GenModel_gemini_1_5_flash_002 = gen.Model{
	Provider:                Provider,
	Name:                    "gemini-1.5-flash-001",
	Description:             "Fast and versatile performance across a diverse variety of tasks",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}

//var GenModel_gemini_1_5_flash_8b = gen.Model{
//	Provider:                Provider,
//	Name:                    "gemini-1.5-flash-8b",
//	Description:             "High volume and lower intelligence tasks",
//	InputContentTypes:       nil,
//	InputMaxToken:           0,
//	OutputMaxToken:          0,
//	SupportTools:            false,
//	SupportStructuredOutput: false,
//}
//var GenModel_gemini_1_5_flash_8b_001 = gen.Model{
//	Provider:                Provider,
//	Name:                    "gemini-1.5-flash-8b-001",
//	Description:             "High volume and lower intelligence tasks",
//	InputContentTypes:       nil,
//	InputMaxToken:           0,
//	OutputMaxToken:          0,
//	SupportTools:            false,
//	SupportStructuredOutput: false,
//}

//var GenModel_gemini_1_5_pro_latest = bellman.GenModel{Name: "gemini-1.5-pro-latest",}

var GenModel_gemini_1_5_pro = gen.Model{
	Provider:                Provider,
	Name:                    "gemini-1.5-pro",
	Description:             "Complex reasoning tasks requiring more intelligence",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}
var GenModel_gemini_1_5_pro_002 = gen.Model{
	Provider:                Provider,
	Name:                    "gemini-1.5-pro-002",
	Description:             "Complex reasoning tasks requiring more intelligence",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}
var GenModel_gemini_1_5_pro_001 = gen.Model{
	Provider:                Provider,
	Name:                    "gemini-1.5-pro-001",
	Description:             "Complex reasoning tasks requiring more intelligence",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}

// https://cloud.google.com/vertex-ai/generative-ai/docs/embeddings/get-text-embeddings#supported-models

var EmbedModel_text_005 = embed.Model{
	Provider:         Provider,
	Name:             "text-embedding-005",
	Description:      "see https://cloud.google.com/vertex-ai/generative-ai/docs/model-reference/text-embeddings-api",
	InputMaxTokens:   2048,
	OutputDimensions: 768,
}
var EmbedModel_text_004 = embed.Model{
	Provider:         Provider,
	Name:             "text-embedding-004",
	Description:      "see https://cloud.google.com/vertex-ai/generative-ai/docs/model-reference/text-embeddings-api",
	InputMaxTokens:   2048,
	OutputDimensions: 768,
}
var EmbedMode_multilang_002 = embed.Model{
	Provider:         Provider,
	Name:             "text-multilingual-embedding-002",
	Description:      "see https://cloud.google.com/vertex-ai/generative-ai/docs/model-reference/text-embeddings-api",
	InputMaxTokens:   2048,
	OutputDimensions: 768,
}

// var EmbedModel_text_gecko_001 = embed.Model{  // deprecated?
//
//		Provider:         Provider,
//		Name:             "textembedding-gecko@001",
//		Description:      "see https://cloud.google.com/vertex-ai/generative-ai/docs/model-reference/text-embeddings-api",
//		InputMaxTokens:   2048,
//		OutputDimensions: 768,
//	}
var EmbedModel_text_gecko_003 = embed.Model{
	Provider:         Provider,
	Name:             "textembedding-gecko@003",
	Description:      "see https://cloud.google.com/vertex-ai/generative-ai/docs/model-reference/text-embeddings-api",
	InputMaxTokens:   2048,
	OutputDimensions: 768,
}

var EmbedModel_text_gecko_multilang_001 = embed.Model{
	Provider:         Provider,
	Name:             "textembedding-gecko-multilingual@001",
	Description:      "see https://cloud.google.com/vertex-ai/generative-ai/docs/model-reference/text-embeddings-api",
	InputMaxTokens:   2048,
	OutputDimensions: 768,
}

const EmbedDimensions = 768

type EmbedType string

const EmbedTypeQuery EmbedType = "RETRIEVAL_QUERY"
const EmbedTypeDocument EmbedType = "RETRIEVAL_DOCUMENT"
const EmbedTypeSimilarity EmbedType = "SEMANTIC_SIMILARITY"
const EmbedTypeClassification EmbedType = "CLASSIFICATION"
const EmbedTypeClustring EmbedType = "CLUSTERING"
const EmbedTypeQA EmbedType = "QUESTION_ANSWERING"
const EmbedTypeVerification EmbedType = "FACT_VERIFICATION"
const EmbedTypeCode EmbedType = "CODE_RETRIEVAL_QUERY"

var EmbedModels = map[string]embed.Model{
	EmbedModel_text_005.Name:     EmbedModel_text_005,
	EmbedModel_text_004.Name:     EmbedModel_text_004,
	EmbedMode_multilang_002.Name: EmbedMode_multilang_002,
	//EmbedModel_text_gecko_001.Name: EmbedModel_text_gecko_001,  // deprecated?
	EmbedModel_text_gecko_003.Name:           EmbedModel_text_gecko_003,
	EmbedModel_text_gecko_multilang_001.Name: EmbedModel_text_gecko_multilang_001,
}

var GenModels = map[string]gen.Model{
	GenModel_gemini_2_0_flash_001.Name: GenModel_gemini_2_0_flash_001,
	GenModel_gemini_1_5_flash.Name:     GenModel_gemini_1_5_flash,
	GenModel_gemini_1_5_flash_001.Name: GenModel_gemini_1_5_flash_001,
	GenModel_gemini_1_5_flash_002.Name: GenModel_gemini_1_5_flash_002,
	GenModel_gemini_1_5_pro.Name:       GenModel_gemini_1_5_pro,
	GenModel_gemini_1_5_pro_002.Name:   GenModel_gemini_1_5_pro_002,
	GenModel_gemini_1_5_pro_001.Name:   GenModel_gemini_1_5_pro_001,
}
