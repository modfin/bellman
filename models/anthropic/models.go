package anthropic

import "github.com/modfin/bellman"

const AnthropicVersion = "2023-06-01"

//type GenModel string

// https://docs.anthropic.com/en/docs/about-claude/models
var GenModel_3_5_sonnet_latest = bellman.GenModel{
	Name:                    "claude-3-5-sonnet-latest",
	Description:             "",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}
var GenModel_3_5_sonnet_20241022 = bellman.GenModel{
	Name:                    "claude-3-5-sonnet-20241022",
	Description:             "",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}
var GenModel_3_5_sonnet_20240620 = bellman.GenModel{
	Name:                    "claude-3-5-sonnet-20240620",
	Description:             "",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}
var GenModel_3_sonnet_20241022 = bellman.GenModel{
	Name:                    "claude-3-sonnet-20240229",
	Description:             "",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}

var GenModel_3_5_haiku_latest = bellman.GenModel{
	Name:                    "claude-3-5-haiku-latest",
	Description:             "",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}
var GenModel_3_5_haiku_20241022 = bellman.GenModel{
	Name:                    "claude-3-5-haiku-20241022",
	Description:             "",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}
var GenModel_3_haiku_20240307 = bellman.GenModel{
	Name:                    "claude-3-haiku-20240307",
	Description:             "",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}

var GenModel_3_opus_latest = bellman.GenModel{
	Name:                    "claude-3-opus-latest",
	Description:             "",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}
var GenModel_3_opus_20240229 = bellman.GenModel{
	Name:                    "claude-3-opus-20240229",
	Description:             "",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}
