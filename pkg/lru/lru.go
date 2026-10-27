// Package lru implements a concurrency-safe bounded least-recently-used cache.
package lru

import (
	"container/list"
	"sync"
)

type entry[K comparable, V any] struct {
	key   K
	value V
}
type Cache[K comparable, V any] struct {
	mu       sync.Mutex
	capacity int
	items    map[K]*list.Element
	order    *list.List
}

func New[K comparable, V any](capacity int) *Cache[K, V] {
	if capacity < 1 {
		panic("lru: capacity must be positive")
	}
	return &Cache[K, V]{capacity: capacity, items: make(map[K]*list.Element), order: list.New()}
}
func (c *Cache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[key]; ok {
		c.order.MoveToFront(el)
		return el.Value.(entry[K, V]).value, true
	}
	var zero V
	return zero, false
}
func (c *Cache[K, V]) Put(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[key]; ok {
		el.Value = entry[K, V]{key, value}
		c.order.MoveToFront(el)
		return
	}
	c.items[key] = c.order.PushFront(entry[K, V]{key, value})
	if c.order.Len() > c.capacity {
		el := c.order.Back()
		delete(c.items, el.Value.(entry[K, V]).key)
		c.order.Remove(el)
	}
}
func (c *Cache[K, V]) Len() int { c.mu.Lock(); defer c.mu.Unlock(); return c.order.Len() }
