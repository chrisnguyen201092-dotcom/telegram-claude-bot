package claude

import (
	"context"
	"sync"
	"time"
)

type RateLimitResult struct {
	Allowed       bool
	RetryAfterSec int
	Reason        string
}

type RateLimiter struct {
	mu            sync.Mutex
	windows       map[string][]time.Time // telegramID -> request timestamps
	active        map[string]bool        // telegramID -> has active query
	limit         int
	windowDur     time.Duration
	maxConcurrent int
}

func NewRateLimiter(limitPerMinute, maxConcurrent int) *RateLimiter {
	return &RateLimiter{
		windows:       make(map[string][]time.Time),
		active:        make(map[string]bool),
		limit:         limitPerMinute,
		windowDur:     time.Minute,
		maxConcurrent: maxConcurrent,
	}
}

func (rl *RateLimiter) CheckRateLimit(telegramID string) RateLimitResult {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Check concurrent query (1 per user)
	if rl.active[telegramID] {
		return RateLimitResult{
			Allowed: false,
			Reason:  "query already in progress",
		}
	}

	now := time.Now()
	cutoff := now.Add(-rl.windowDur)

	// Prune expired timestamps
	ts := rl.windows[telegramID]
	valid := ts[:0]
	for _, t := range ts {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	rl.windows[telegramID] = valid

	if len(valid) >= rl.limit {
		// Earliest timestamp in window determines retry-after
		oldest := valid[0]
		retryAfter := int(oldest.Add(rl.windowDur).Sub(now).Seconds()) + 1
		if retryAfter < 1 {
			retryAfter = 1
		}
		return RateLimitResult{
			Allowed:       false,
			RetryAfterSec: retryAfter,
			Reason:        "rate limit exceeded",
		}
	}

	// Record this request
	rl.windows[telegramID] = append(rl.windows[telegramID], now)
	return RateLimitResult{Allowed: true}
}

func (rl *RateLimiter) MarkActive(telegramID string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.active[telegramID] = true
}

func (rl *RateLimiter) MarkInactive(telegramID string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.active, telegramID)
}

func (rl *RateLimiter) IsActive(telegramID string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	return rl.active[telegramID]
}

func (rl *RateLimiter) GetActiveCount() int {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	return len(rl.active)
}

func (rl *RateLimiter) StartCleanup(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				rl.mu.Lock()
				cutoff := time.Now().Add(-rl.windowDur)
				for id, ts := range rl.windows {
					valid := ts[:0]
					for _, t := range ts {
						if t.After(cutoff) {
							valid = append(valid, t)
						}
					}
					if len(valid) == 0 {
						delete(rl.windows, id)
					} else {
						rl.windows[id] = valid
					}
				}
				rl.mu.Unlock()
			}
		}
	}()
}
