package jpf

import (
	"slices"
	"sync"
)

// NewInMemoryCache creates an in-memory implementation of ModelResponseCache.
// It stores model responses in memory using a hash of the input messages as a key.
func NewInMemoryCache() Cache {
	return &inMemoryCache{
		entries: make(map[string][]byte),
		lock:    &sync.Mutex{},
	}
}

type inMemoryCache struct {
	lock    *sync.Mutex
	entries map[string][]byte
}

func (c *inMemoryCache) Set(key string, data []byte) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.entries[key] = slices.Clone(data)
	return nil
}
func (c *inMemoryCache) Get(key string) ([]byte, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	val, ok := c.entries[key]
	if !ok {
		return nil, ErrNoCache
	}
	return slices.Clone(val), nil
}
