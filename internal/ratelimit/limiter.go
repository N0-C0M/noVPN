package ratelimit

import "sync"

type Limiter interface {
	AllowConnection(key string) bool
	DoneConnection(key string)
	AllowPacket(key string, size int) bool
}

type NoopLimiter struct{}

func (NoopLimiter) AllowConnection(string) bool { return true }
func (NoopLimiter) DoneConnection(string)       {}
func (NoopLimiter) AllowPacket(string, int) bool {
	return true
}

type MemoryLimiter struct {
	mu       sync.Mutex
	total    int
	perKey   map[string]int
	maxTotal int
	maxPerIP int
}

func NewMemoryLimiter(maxTotal, maxPerIP int) *MemoryLimiter {
	return &MemoryLimiter{
		perKey:   make(map[string]int),
		maxTotal: maxTotal,
		maxPerIP: maxPerIP,
	}
}

func (l *MemoryLimiter) AllowConnection(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.maxTotal > 0 && l.total >= l.maxTotal {
		return false
	}
	if l.maxPerIP > 0 && l.perKey[key] >= l.maxPerIP {
		return false
	}

	l.total++
	l.perKey[key]++
	return true
}

func (l *MemoryLimiter) DoneConnection(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.total > 0 {
		l.total--
	}

	count := l.perKey[key]
	switch {
	case count <= 1:
		delete(l.perKey, key)
	default:
		l.perKey[key] = count - 1
	}
}

func (l *MemoryLimiter) AllowPacket(string, int) bool {
	return true
}
