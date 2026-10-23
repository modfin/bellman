package gen

// ThinkingLevel controls how much a model reasons before answering. Providers translate it into
// their own representation, e.g. reasoning effort, thinking level or a token budget, and round it
// to the nearest level the model supports.
type ThinkingLevel string

const (
	ThinkingLevelNone    ThinkingLevel = "none"
	ThinkingLevelMinimal ThinkingLevel = "minimal"
	ThinkingLevelLow     ThinkingLevel = "low"
	ThinkingLevelMedium  ThinkingLevel = "medium"
	ThinkingLevelHigh    ThinkingLevel = "high"
	ThinkingLevelXHigh   ThinkingLevel = "xhigh"
	ThinkingLevelMax     ThinkingLevel = "max"
)

// ThinkingLevels lists every level, ordered from least to most reasoning.
var ThinkingLevels = []ThinkingLevel{
	ThinkingLevelNone,
	ThinkingLevelMinimal,
	ThinkingLevelLow,
	ThinkingLevelMedium,
	ThinkingLevelHigh,
	ThinkingLevelXHigh,
	ThinkingLevelMax,
}

func (l ThinkingLevel) rank() int {
	for i, level := range ThinkingLevels {
		if level == l {
			return i
		}
	}
	return -1
}

func (l ThinkingLevel) Valid() bool {
	return l.rank() >= 0
}

// Resolve returns the level in supported closest to l. It rounds down to the nearest supported
// level and only rounds up when nothing lower is supported. An unset or unknown level, or an
// empty supported list, returns l unchanged.
func (l ThinkingLevel) Resolve(supported []ThinkingLevel) ThinkingLevel {
	r := l.rank()
	if r < 0 || len(supported) == 0 {
		return l
	}
	below, above := ThinkingLevel(""), ThinkingLevel("")
	for _, s := range supported {
		sr := s.rank()
		switch {
		case sr < 0:
		case sr == r:
			return s
		case sr < r && (below == "" || sr > below.rank()):
			below = s
		case sr > r && (above == "" || sr < above.rank()):
			above = s
		}
	}
	if below != "" {
		return below
	}
	if above != "" {
		return above
	}
	return l
}
