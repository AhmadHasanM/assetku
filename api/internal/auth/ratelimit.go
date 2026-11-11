package auth

import (
	"strings"
	"sync"
	"time"
)

// RateLimiter caps failed login attempts per key within a sliding window. In-memory.
type RateLimiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	now    func() time.Time
	fails  map[string][]time.Time
}

func NewRateLimiter(max int, window time.Duration) *RateLimiter {
	return &RateLimiter{max: max, window: window, now: time.Now, fails: map[string][]time.Time{}}
}

// LoginKey builds the limiter key from client IP and email.
func LoginKey(ip, email string) string { return ip + "|" + strings.ToLower(strings.TrimSpace(email)) }

func (l *RateLimiter) recent(key string) []time.Time {
	cut := l.now().Add(-l.window)
	ts := l.fails[key]
	i := 0
	for i < len(ts) && !ts[i].After(cut) {
		i++
	}
	ts = ts[i:]
	if len(ts) == 0 {
		delete(l.fails, key)
		return nil
	}
	l.fails[key] = ts
	return ts
}

// Allowed reports whether another attempt is permitted for key.
func (l *RateLimiter) Allowed(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.recent(key)) < l.max
}

// Fail records a failed attempt.
func (l *RateLimiter) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.fails) > 10000 { // bound memory: drop expired keys
		for k := range l.fails {
			l.recent(k)
		}
	}
	l.fails[key] = append(l.recent(key), l.now())
}

// Reset clears the failures for key (after a successful login).
func (l *RateLimiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fails, key)
}
