package main

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type RateLimitConfig struct {
	BurstTokens     int           `json:"burst_tokens"`
	BurstWindow     time.Duration `json:"burst_window"`
	SustainedTokens int           `json:"sustained_tokens"`
	SustainedWindow time.Duration `json:"sustained_window"`
}

type keyLimiter struct {
	config RateLimitConfig

	sustainedTokens         float64
	lastSustainedRefillTime time.Time
	sustainedRefillDuration time.Duration

	burstTokens         float64
	lastBurstRefillTime time.Time
	burstRefillRate     float64 // tokens per second

	mu sync.Mutex
}

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
		// Calculate burst refill rate
		var burstRefillRate float64
		if config.BurstWindow > 0 && config.BurstTokens > 0 {
			burstRefillRate = float64(config.BurstTokens) / config.BurstWindow.Seconds()
		}

		kl := &keyLimiter{
			config:                  config,
			sustainedTokens:         float64(config.SustainedTokens),
			lastSustainedRefillTime: time.Now(),
			sustainedRefillDuration: config.SustainedWindow,
			burstTokens:             float64(config.BurstTokens),
			lastBurstRefillTime:     time.Now(),
			burstRefillRate:         burstRefillRate,
		}
		rl.limits[keyName] = kl
	}

	return rl
}

func (kl *keyLimiter) refill() {
	now := time.Now()

	// Refill sustained bucket
	if kl.sustainedRefillDuration > 0 {
		elapsed := now.Sub(kl.lastSustainedRefillTime)
		if elapsed > kl.sustainedRefillDuration {
			kl.sustainedTokens = float64(kl.config.SustainedTokens)
			kl.lastSustainedRefillTime = now
		}
	}

	// Refill burst bucket
	if kl.burstRefillRate > 0 {
		elapsed := now.Sub(kl.lastBurstRefillTime).Seconds()
		if elapsed > 0 {
			tokensToAdd := elapsed * kl.burstRefillRate
			kl.burstTokens += tokensToAdd
			// Cap burst tokens at its maximum capacity.
			if kl.burstTokens > float64(kl.config.BurstTokens) {
				kl.burstTokens = float64(kl.config.BurstTokens)
			}
			kl.lastBurstRefillTime = now
		}
	}
}

func (rl *RateLimiter) HasCapacity(keyName string) bool {
	if rl.disabled {
		return true
	}

	rl.mu.RLock()
	limiter, exists := rl.limits[keyName]
	rl.mu.RUnlock()

	if !exists {
		return true
	}

	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	limiter.refill()

	sustainedHasCapacity := limiter.sustainedTokens >= 1
	burstHasCapacity := limiter.burstTokens >= 1

	return sustainedHasCapacity && burstHasCapacity
}

func (rl *RateLimiter) Consume(keyName string, n int) {
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

	limiter.refill()

	tokensToConsume := float64(n)

	limiter.sustainedTokens -= tokensToConsume
	limiter.burstTokens -= tokensToConsume
}

func ParseRateLimitConfig(jsonData string) (map[string]RateLimitConfig, error) {
	if jsonData == "" {
		return nil, nil
	}

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
