package main

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// RateLimitConfig defines rate limiting parameters for an API key.
type RateLimitConfig struct {
	// Burst limiting (short-term) using a token bucket approach.
	BurstTokens int           `json:"burst_tokens"` // Max tokens (bucket size).
	BurstWindow time.Duration `json:"burst_window"` // Time to refill the bucket completely.

	// Sustained limiting (long-term) using a sliding window.
	SustainedTokens int           `json:"sustained_tokens"` // Max tokens in the sustained window.
	SustainedWindow time.Duration `json:"sustained_window"` // Duration of the sustained window.
}

// tokenEvent tracks when tokens were consumed for the sustained window.
type tokenEvent struct {
	tokens    int
	timestamp time.Time
}

// keyLimiter tracks rate limit state for a single API key.
type keyLimiter struct {
	config RateLimitConfig

	// Burst limiting state (token bucket).
	burstTokens     float64   // Current available tokens (float for precision).
	burstLastRefill time.Time // Timestamp of the last token refill.

	// Sustained limiting state (sliding window).
	sustainedEvents []tokenEvent

	mu sync.Mutex
}

// RateLimiter manages rate limits for multiple API keys.
type RateLimiter struct {
	limits   map[string]*keyLimiter
	mu       sync.RWMutex
	disabled bool
}

// NewRateLimiter creates a new rate limiter with the given configurations.
func NewRateLimiter(configs map[string]RateLimitConfig) *RateLimiter {
	if configs == nil || len(configs) == 0 {
		return &RateLimiter{disabled: true}
	}

	rl := &RateLimiter{
		limits: make(map[string]*keyLimiter),
	}

	for keyName, config := range configs {
		kl := &keyLimiter{
			config:          config,
			burstTokens:     float64(config.BurstTokens), // Start with a full bucket.
			burstLastRefill: time.Now(),
			sustainedEvents: make([]tokenEvent, 0),
		}
		rl.limits[keyName] = kl
	}

	return rl
}

// refillBurstTokens calculates and adds tokens to the burst bucket based on elapsed time.
// This function is NOT thread-safe and must be called within a locked mutex.
func (kl *keyLimiter) refillBurstTokens() {
	now := time.Now()
	if kl.config.BurstTokens <= 0 || kl.config.BurstWindow <= 0 {
		return
	}

	// Calculate time elapsed since the last refill.
	elapsed := now.Sub(kl.burstLastRefill)
	if elapsed <= 0 {
		return
	}

	// Calculate the rate of token refill per second.
	refillRate := float64(kl.config.BurstTokens) / kl.config.BurstWindow.Seconds()

	// Calculate how many tokens to add.
	tokensToAdd := elapsed.Seconds() * refillRate

	// Add the new tokens and cap at the maximum bucket size.
	kl.burstTokens += tokensToAdd
	if kl.burstTokens > float64(kl.config.BurstTokens) {
		kl.burstTokens = float64(kl.config.BurstTokens)
	}

	// Update the last refill time.
	kl.burstLastRefill = now
}

// Consume records token usage after a successful request.
// It's designed to be called when you are certain the operation will proceed.
func (rl *RateLimiter) Consume(keyName string, tokens int) {
	if rl.disabled {
		return
	}

	rl.mu.RLock()
	limiter, exists := rl.limits[keyName]
	rl.mu.RUnlock()

	if !exists {
		return
	}

	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	// Refill burst bucket before consuming to get the most up-to-date token count.
	if limiter.config.BurstTokens > 0 {
		limiter.refillBurstTokens()
		limiter.burstTokens -= float64(tokens)
	}

	// Add to sustained window tracking.
	if limiter.config.SustainedTokens > 0 && limiter.config.SustainedWindow > 0 {
		limiter.sustainedEvents = append(limiter.sustainedEvents, tokenEvent{
			tokens:    tokens,
			timestamp: time.Now(),
		})
		// Prune old events to prevent memory leak
		limiter.pruneSustainedEvents()
	}
}

// HasCapacity checks if a key has any remaining capacity.
// It denies a request if the burst bucket has less than 1 token or if the
// sustained limit has been met or exceeded.
func (rl *RateLimiter) HasCapacity(keyName string) bool {
	if rl.disabled {
		return true
	}

	rl.mu.RLock()
	limiter, exists := rl.limits[keyName]
	rl.mu.RUnlock()

	// If no rate limit is configured for this key, allow it.
	if !exists {
		return true
	}

	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	if limiter.config.BurstTokens > 0 {
		// Refill tokens before checking capacity.
		limiter.refillBurstTokens()
		// Check if there is at least 1 token available.
		if limiter.burstTokens < 1 {
			return false
		}
	}

	if limiter.config.SustainedTokens > 0 && limiter.config.SustainedWindow > 0 {
		limiter.pruneSustainedEvents()

		// Calculate current usage in the window.
		sustainedTotal := 0
		for _, event := range limiter.sustainedEvents {
			sustainedTotal += event.tokens
		}

		// Check if the sustained limit has been reached or exceeded.
		if sustainedTotal >= limiter.config.SustainedTokens {
			return false
		}
	}

	return true
}

// pruneSustainedEvents removes events that are outside the sustained window.
// This function is NOT thread-safe and must be called within a locked mutex.
func (kl *keyLimiter) pruneSustainedEvents() {
	if kl.config.SustainedWindow <= 0 {
		return
	}

	now := time.Now()
	windowStart := now.Add(-kl.config.SustainedWindow)

	// Find the first event that is still within the window.
	firstValidIndex := 0
	for i, event := range kl.sustainedEvents {
		if !event.timestamp.Before(windowStart) {
			firstValidIndex = i
			break
		}
		// If the last event is before window start, all can be cleared.
		if i == len(kl.sustainedEvents)-1 {
			firstValidIndex = len(kl.sustainedEvents)
		}
	}

	// Slice the array to remove old events.
	if firstValidIndex > 0 {
		kl.sustainedEvents = kl.sustainedEvents[firstValidIndex:]
	}
}

// ParseRateLimitConfig parses rate limit configuration from JSON.
func ParseRateLimitConfig(jsonData string) (map[string]RateLimitConfig, error) {
	if jsonData == "" {
		return nil, nil
	}

	// A temporary struct to help with parsing duration strings from JSON.
	var rawConfig map[string]struct {
		BurstTokens     int    `json:"burst_tokens"`
		BurstWindow     string `json:"burst_window"`
		SustainedTokens int    `json:"sustained_tokens"`
		SustainedWindow string `json:"sustained_window"`
	}

	err := json.Unmarshal([]byte(jsonData), &rawConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to parse rate limit config: %w", err)
	}

	configs := make(map[string]RateLimitConfig)
	for keyName, raw := range rawConfig {
		config := RateLimitConfig{
			BurstTokens:     raw.BurstTokens,
			SustainedTokens: raw.SustainedTokens,
		}

		if raw.BurstWindow != "" {
			config.BurstWindow, err = time.ParseDuration(raw.BurstWindow)
			if err != nil {
				return nil, fmt.Errorf("invalid burst_window for key %s: %w", keyName, err)
			}
		}

		if raw.SustainedWindow != "" {
			config.SustainedWindow, err = time.ParseDuration(raw.SustainedWindow)
			if err != nil {
				return nil, fmt.Errorf("invalid sustained_window for key %s: %w", keyName, err)
			}
		}

		configs[keyName] = config
	}

	return configs, nil
}
