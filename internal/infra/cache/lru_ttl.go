package cache

import (
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
)

type entry[V any] struct {
	value     V
	expiresAt time.Time
}

// LRUCache implementa um cache LRU com TTL por item.
type LRUCache[K comparable, V any] struct {
	mu  sync.Mutex
	lru *lru.Cache[K, entry[V]]
	now func() time.Time
}

// NewLRUCache cria um cache LRU com tamanho máximo.
func NewLRUCache[K comparable, V any](maxSize int) (*LRUCache[K, V], error) {
	if maxSize <= 0 {
		maxSize = 100
	}
	c, err := lru.New[K, entry[V]](maxSize)
	if err != nil {
		return nil, err
	}
	return &LRUCache[K, V]{lru: c, now: time.Now}, nil
}

func (c *LRUCache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var zero V
	e, ok := c.lru.Get(key)
	if !ok {
		return zero, false
	}
	if !e.expiresAt.IsZero() && c.now().After(e.expiresAt) {
		c.lru.Remove(key)
		return zero, false
	}
	return e.value, true
}

func (c *LRUCache[K, V]) Set(key K, value V, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = c.now().Add(ttl)
	}
	c.lru.Add(key, entry[V]{value: value, expiresAt: expiresAt})
}

