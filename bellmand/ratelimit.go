package main

import (
	"sync"
	"time"
)

type RateLimitConfig struct {
	BurstTokens         int           `json:"burst_tokens"`
	BurstWindow         string        `json:"burst_window"`
	BurstWindowDuration time.Duration `json:"-"`

	SustainedTokens         int           `json:"sustained_tokens"`
	SustainedWindow         string        `json:"sustained_window"`
	SustainedWindowDuration time.Duration `json:"-"`
}

type keyLimiter struct {
	config RateLimitConfig

	sustainedTokens         float64
	lastSustainedRefillTime time.Time
	sustainedWindowDuration time.Duration

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

func NewRateLimiter(apiKeyConfigs map[string]ApiKeyConfig) (*RateLimiter, error) {
	var configsWithLimits = make(map[string]RateLimitConfig)
	for _, apiKeyConfig := range apiKeyConfigs {
		if apiKeyConfig.RateLimit != nil {
			configsWithLimits[apiKeyConfig.Id] = *apiKeyConfig.RateLimit
		}
	}
	if configsWithLimits == nil || len(configsWithLimits) == 0 {
		return &RateLimiter{disabled: true}, nil
	}

	rl := &RateLimiter{
		limits: make(map[string]*keyLimiter),
	}

	for keyId, config := range configsWithLimits {
		// Calculate burst refill rate
		var burstRefillRate float64
		if config.BurstWindow != "" && config.BurstTokens > 0 {
			burstWindowDuration, err := time.ParseDuration(config.BurstWindow)
			if err != nil {
				return nil, err
			}
			burstRefillRate = float64(config.BurstTokens) / burstWindowDuration.Seconds()
		}
		var sustainedWindowDuration time.Duration
		if config.SustainedWindow != "" {
			_sustainedWindowDuration, err := time.ParseDuration(config.SustainedWindow)
			if err != nil {
				return nil, err
			}
			sustainedWindowDuration = _sustainedWindowDuration
		}

		kl := &keyLimiter{
			config:                  config,
			sustainedTokens:         float64(config.SustainedTokens),
			lastSustainedRefillTime: time.Now(),
			sustainedWindowDuration: sustainedWindowDuration,
			burstTokens:             float64(config.BurstTokens),
			lastBurstRefillTime:     time.Now(),
			burstRefillRate:         burstRefillRate,
		}
		rl.limits[keyId] = kl
	}

	return rl, nil
}

func (kl *keyLimiter) refill() {
	now := time.Now()

	// Refill sustained bucket
	if kl.sustainedWindowDuration > 0 {
		elapsed := now.Sub(kl.lastSustainedRefillTime)
		if elapsed > kl.sustainedWindowDuration {
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

func (rl *RateLimiter) HasCapacity(keyId string) bool {
	if rl.disabled {
		return true
	}

	rl.mu.RLock()
	limiter, exists := rl.limits[keyId]
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

func (rl *RateLimiter) Consume(keyId string, n int) {
	if rl.disabled {
		return
	}

	rl.mu.RLock()
	limiter, exists := rl.limits[keyId]
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
