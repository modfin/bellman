package openai

import "github.com/modfin/bellman"

// https://platform.openai.com/docs/models#gpt-4o-mini
//type GenModel string

var GenModel_gpt4o_latest = bellman.GenModel{
	Name: "chatgpt-4o-latest",
	Description: "Our high-intelligence flagship Model for complex, multi-step tasks. GPT-4o is cheaper and " +
		"faster than GPT-4 Turbo.",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}
var GenModel_gpt4o = bellman.GenModel{
	Name: "gpt-4o",
	Description: "Our high-intelligence flagship Model for complex, multi-step tasks. GPT-4o is cheaper and " +
		"faster than GPT-4 Turbo.",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}
var GenModel_gpt4o_240806 = bellman.GenModel{
	Name: "gpt-4o-2024-08-06",
	Description: "Our high-intelligence flagship Model for complex, multi-step tasks. GPT-4o is cheaper and " +
		"faster than GPT-4 Turbo.",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}
var GenModel_gpt4o_240513 = bellman.GenModel{
	Name: "gpt-4o-2024-05-13",
	Description: "Our high-intelligence flagship Model for complex, multi-step tasks. GPT-4o is cheaper and " +
		"faster than GPT-4 Turbo.",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}

// GenModel_gpt4o_mini
var GenModel_gpt4o_mini = bellman.GenModel{
	Name: "gpt-4o-mini",
	Description: "Our affordable and intelligent small Model for fast, lightweight tasks. GPT-4o mini is " +
		"cheaper and more capable,than GPT-3.5 Turbo.",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}
var GenModel_gpt4o_mini_240718 = bellman.GenModel{
	Name: "gpt-4o-mini-2024-07-18",
	Description: "Our affordable and intelligent small Model for fast, lightweight tasks. GPT-4o mini is " +
		"cheaper and more capable,than GPT-3.5 Turbo.",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}

// GenModel_o1_preview
var GenModel_o1_preview = bellman.GenModel{
	Name: "o1-preview",
	Description: "The o1 series of large language models are trained with reinforcement learning to perform " +
		"complex reasoning. o1 models think before they answer, producing a long internal chain of thought before " +
		"responding to the user.",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}
var GenModel_o1_preview_240912 = bellman.GenModel{
	Name: "o1-preview-2024-09-12",
	Description: "The o1 series of large language models are trained with reinforcement learning to perform " +
		"complex reasoning. o1 models think before they answer, producing a long internal chain of thought before " +
		"responding to the user.",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}
var GenModel_o1_mini = bellman.GenModel{
	Name: "o1-mini",
	Description: "The o1 series of large language models are trained with reinforcement learning to perform " +
		"complex reasoning. o1 models think before they answer, producing a long internal chain of thought before " +
		"responding to the user.",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}
var GenModel_o1_mini_240912 = bellman.GenModel{
	Name: "o1-mini-2024-09-12",
	Description: "The o1 series of large language models are trained with reinforcement learning to perform " +
		"complex reasoning. o1 models think before they answer, producing a long internal chain of thought before " +
		"responding to the user.",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}

// GenModel_gpt4_turbo
var GenModel_gpt4_turbo = bellman.GenModel{
	Name: "gpt-4-turbo",
	Description: "GPT-4 is a large multimodal Model (accepting text or image inputs and outputting text) " +
		"that can solve difficult problems with greater accuracy than any of our previous models, thanks to its broader " +
		"general knowledge and advanced reasoning capabilities. GPT-4 is available in the OpenAI API to paying customers. " +
		"Like gpt-3.5-turbo, GPT-4 is optimized for chat but works well for traditional completions tasks using the Chat " +
		"Completions API. Learn how to use GPT-4 in our text generation guide.",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}
var GenModel_gpt4_turbo_240409 = bellman.GenModel{
	Name: "gpt-4-turbo-2024-04-09",
	Description: "GPT-4 is a large multimodal Model (accepting text or image inputs and outputting text) " +
		"that can solve difficult problems with greater accuracy than any of our previous models, thanks to its broader " +
		"general knowledge and advanced reasoning capabilities. GPT-4 is available in the OpenAI API to paying customers. " +
		"Like gpt-3.5-turbo, GPT-4 is optimized for chat but works well for traditional completions tasks using the Chat " +
		"Completions API. Learn how to use GPT-4 in our text generation guide.",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}
var GenModel_gpt4_turbo_preview = bellman.GenModel{
	Name: "gpt-4-turbo-preview",
	Description: "GPT-4 is a large multimodal Model (accepting text or image inputs and outputting text) " +
		"that can solve difficult problems with greater accuracy than any of our previous models, thanks to its broader " +
		"general knowledge and advanced reasoning capabilities. GPT-4 is available in the OpenAI API to paying customers. " +
		"Like gpt-3.5-turbo, GPT-4 is optimized for chat but works well for traditional completions tasks using the Chat " +
		"Completions API. Learn how to use GPT-4 in our text generation guide.",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}
var GenModel_gpt4_preview_0125 = bellman.GenModel{
	Name: "gpt-4-0125-preview",
	Description: "GPT-4 is a large multimodal Model (accepting text or image inputs and outputting text) " +
		"that can solve difficult problems with greater accuracy than any of our previous models, thanks to its broader " +
		"general knowledge and advanced reasoning capabilities. GPT-4 is available in the OpenAI API to paying customers. " +
		"Like gpt-3.5-turbo, GPT-4 is optimized for chat but works well for traditional completions tasks using the Chat " +
		"Completions API. Learn how to use GPT-4 in our text generation guide.",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}
var GenModel_gpt4_preview_1106 = bellman.GenModel{
	Name: "gpt-4-1106-preview",
	Description: "GPT-4 is a large multimodal Model (accepting text or image inputs and outputting text) " +
		"that can solve difficult problems with greater accuracy than any of our previous models, thanks to its broader " +
		"general knowledge and advanced reasoning capabilities. GPT-4 is available in the OpenAI API to paying customers. " +
		"Like gpt-3.5-turbo, GPT-4 is optimized for chat but works well for traditional completions tasks using the Chat " +
		"Completions API. Learn how to use GPT-4 in our text generation guide.",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}
var GenModel_gpt4 = bellman.GenModel{
	Name: "gpt-4",
	Description: "GPT-4 is a large multimodal Model (accepting text or image inputs and outputting text) " +
		"that can solve difficult problems with greater accuracy than any of our previous models, thanks to its broader " +
		"general knowledge and advanced reasoning capabilities. GPT-4 is available in the OpenAI API to paying customers. " +
		"Like gpt-3.5-turbo, GPT-4 is optimized for chat but works well for traditional completions tasks using the Chat " +
		"Completions API. Learn how to use GPT-4 in our text generation guide.",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}
var GenModel_gpt4_0613 = bellman.GenModel{
	Name: "gpt-4-0613",
	Description: "GPT-4 is a large multimodal Model (accepting text or image inputs and outputting text) " +
		"that can solve difficult problems with greater accuracy than any of our previous models, thanks to its broader " +
		"general knowledge and advanced reasoning capabilities. GPT-4 is available in the OpenAI API to paying customers. " +
		"Like gpt-3.5-turbo, GPT-4 is optimized for chat but works well for traditional completions tasks using the Chat " +
		"Completions API. Learn how to use GPT-4 in our text generation guide.",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}

// https://platform.openai.com/docs/models#embeddings

var EmbedModel_text3_small = bellman.EmbedModel{
	Name:             "text-embedding-3-small",
	Description:      "Most capable embedding Model for both english and non-english tasks",
	InputMaxTokens:   8191,
	OutputDimensions: 1536,
}
var EmbedModel_text3_large = bellman.EmbedModel{
	Name:             "text-embedding-3-large",
	Description:      "Increased performance over 2nd generation ada embedding Model",
	InputMaxTokens:   8191,
	OutputDimensions: 3072,
}
var EmbedModel_text_ada_002 = bellman.EmbedModel{
	Name:             "text-embedding-ada-002",
	Description:      "Most capable 2nd generation embedding Model, replacing 16 first generation models",
	InputMaxTokens:   8191,
	OutputDimensions: 1536,
}

var EmbedModels = map[string]bellman.EmbedModel{
	EmbedModel_text3_small.Name:  EmbedModel_text3_small,
	EmbedModel_text3_large.Name:  EmbedModel_text3_large,
	EmbedModel_text_ada_002.Name: EmbedModel_text_ada_002,
}

var GenModels = map[string]bellman.GenModel{
	GenModel_gpt4o_latest.Name:       GenModel_gpt4o_latest,
	GenModel_gpt4o.Name:              GenModel_gpt4o,
	GenModel_gpt4o_240806.Name:       GenModel_gpt4o_240806,
	GenModel_gpt4o_240513.Name:       GenModel_gpt4o_240513,
	GenModel_gpt4o_mini.Name:         GenModel_gpt4o_mini,
	GenModel_gpt4o_mini_240718.Name:  GenModel_gpt4o_mini_240718,
	GenModel_o1_preview.Name:         GenModel_o1_preview,
	GenModel_o1_preview_240912.Name:  GenModel_o1_preview_240912,
	GenModel_o1_mini.Name:            GenModel_o1_mini,
	GenModel_o1_mini_240912.Name:     GenModel_o1_mini_240912,
	GenModel_gpt4_turbo.Name:         GenModel_gpt4_turbo,
	GenModel_gpt4_turbo_240409.Name:  GenModel_gpt4_turbo_240409,
	GenModel_gpt4_turbo_preview.Name: GenModel_gpt4_turbo_preview,
	GenModel_gpt4_preview_0125.Name:  GenModel_gpt4_preview_0125,
	GenModel_gpt4_preview_1106.Name:  GenModel_gpt4_preview_1106,
	GenModel_gpt4.Name:               GenModel_gpt4,
	GenModel_gpt4_0613.Name:          GenModel_gpt4_0613,
}
