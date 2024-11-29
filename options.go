package bellman

import "github.com/modfin/bellman/tools"

type GeneratorOption func(generator Generator) Generator

func WithModel(model GenModel) GeneratorOption {
	return func(g Generator) Generator {
		return g.Model(model)
	}
}

func WithTools(tools ...tools.Tool) GeneratorOption {
	return func(g Generator) Generator {
		return g.Tools(tools...)
	}
}

func WithTool(tool tools.Tool) GeneratorOption {
	return func(g Generator) Generator {
		return g.Tool(tool)
	}
}

func WithSystem(prompt string) GeneratorOption {
	return func(g Generator) Generator {
		return g.System(prompt)
	}
}

func WithOutput(element any) GeneratorOption {
	return func(g Generator) Generator {
		return g.Output(element)
	}
}

func WithStopAt(stop ...string) GeneratorOption {
	return func(g Generator) Generator {
		return g.StopAt(stop...)
	}
}

func WithTemperature(temperature float64) GeneratorOption {
	return func(g Generator) Generator {
		return g.Temperature(temperature)
	}
}

func WithTopP(topP float64) GeneratorOption {
	return func(g Generator) Generator {
		return g.TopP(topP)
	}
}

func WithMaxTokens(maxTokens int) GeneratorOption {
	return func(g Generator) Generator {
		return g.MaxTokens(maxTokens)
	}
}
