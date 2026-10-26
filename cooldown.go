package main

import (
	"math"
	"sync"
	"time"
)

// cooldownManager is an in-memory per-user per-handler cooldown store,
// mirroring the python bot's decorator behaviour.
type cooldownManager struct {
	mu        sync.Mutex
	cooldowns map[int64]map[string]time.Time
}

func newCooldownManager() *cooldownManager {
	return &cooldownManager{cooldowns: map[int64]map[string]time.Time{}}
}

// Check returns 0 if the user may proceed (and records the invocation),
// otherwise the number of seconds still remaining.
func (c *cooldownManager) Check(userID int64, handler string, seconds int) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cooldowns[userID] == nil {
		c.cooldowns[userID] = map[string]time.Time{}
	}

	now := time.Now()
	if last, ok := c.cooldowns[userID][handler]; ok {
		elapsed := now.Sub(last).Seconds()
		if elapsed < float64(seconds) {
			remaining := int(math.Round(float64(seconds) - elapsed))
			if remaining < 1 {
				remaining = 1
			}
			logger.Debug("rate limited", "handler", handler, "user_id", userID, "retry_in_s", remaining)
			return remaining
		}
	}

	c.cooldowns[userID][handler] = now
	return 0
}
