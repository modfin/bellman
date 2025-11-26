package tools

type WebSearchTool struct {
	Enabled         bool                  `json:"enabled"`
	AllowedDomains  []string              `json:"allowed_domains,omitempty"`
	ExcludedDomains []string              `json:"excluded_domains,omitempty"`
	UserLocation    WebSearchUserLocation `json:"location,omitempty"`
}

type WebSearchUserLocation struct {
	Country   string  `json:"country,omitempty"`
	Region    string  `json:"region,omitempty"`
	City      string  `json:"city,omitempty"`
	Timezone  string  `json:"timezone,omitempty"`
	Longitude float64 `json:"longitude,omitempty"`
	Latitude  float64 `json:"latitude,omitempty"`
}
type WebSearchToolOption func(tool WebSearchTool) WebSearchTool

func WithWebSearchAllowedDomains(domains []string) WebSearchToolOption {
	return func(tool WebSearchTool) WebSearchTool {
		tool.AllowedDomains = domains
		return tool
	}
}

func WithWebSearchExcludedDomains(domains []string) WebSearchToolOption {
	return func(tool WebSearchTool) WebSearchTool {
		tool.ExcludedDomains = domains
		return tool
	}
}

func WithWebSearchUserLocation(location WebSearchUserLocation) WebSearchToolOption {
	return func(tool WebSearchTool) WebSearchTool {
		tool.UserLocation = location
		return tool
	}
}

var UserLocationUSNewYork = WebSearchUserLocation{
	Country:   "US",
	Region:    "New York",
	City:      "New York",
	Timezone:  "America/New_York",
	Longitude: -74.0060,
	Latitude:  40.7128,
}

var UserLocationGBLondon = WebSearchUserLocation{
	Country:   "GB",
	Region:    "England",
	City:      "London",
	Timezone:  "Europe/London",
	Longitude: -0.1276,
	Latitude:  51.5074,
}

var UserLocationDEBerlin = WebSearchUserLocation{
	Country:   "DE",
	Region:    "Berlin",
	City:      "Berlin",
	Timezone:  "Europe/Berlin",
	Longitude: 13.4050,
	Latitude:  52.5200,
}

func NewWebSearchTool(options ...WebSearchToolOption) WebSearchTool {
	t := WebSearchTool{
		Enabled: true,
	}
	for _, opt := range options {
		t = opt(t)
	}
	return t
}
