package typesafe

import "github.com/modfin/bellman/models/decide"

const Provider = "TypeSafe"

// https://docs.typesafe.ai
//
// Jev is a "System One" decision model: it answers typed questions against a
// state and never generates text. Text and JSON state only, 32k context,
// billed on input tokens.

var DecideModel_jev_latest = decide.Model{
	Provider:         Provider,
	Name:             "jev-latest",
	InputMaxTokens:   32_000,
	SupportJSONState: true,
	Description:      "Alias for the newest published Jev build.",
}

var DecideModel_jev_1_13 = decide.Model{
	Provider:         Provider,
	Name:             "jev-1.13.0",
	InputMaxTokens:   32_000,
	SupportJSONState: true,
}
