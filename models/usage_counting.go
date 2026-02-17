package models

import (
	"context"
	"sync"

	"github.com/JoshPattman/jpf"
)

// NewUsageCountingModel wraps a Model with token usage tracking functionality.
// It aggregates token usage statistics in the provided UsageCounter,
// which allows monitoring total token consumption across multiple model calls.
func NewUsageCountingModel(model jpf.Model, counter *UsageCounter) jpf.Model {
	return &usageCountingModel{
		counter: counter,
		model:   model,
	}
}

// Counts up the sum usage.
// Is completely concurrent-safe.
type UsageCounter struct {
	usage jpf.Usage
	lock  *sync.Mutex
}

// NewUsageCounter creates a new UsageCounter with zero initial usage.
// The counter is safe for concurrent use across multiple goroutines.
func NewUsageCounter() *UsageCounter {
	return &UsageCounter{
		usage: jpf.Usage{},
		lock:  &sync.Mutex{},
	}
}

// Add the given usage to the counter.
func (u *UsageCounter) Add(usage jpf.Usage) {
	u.lock.Lock()
	defer u.lock.Unlock()
	u.usage = u.usage.Add(usage)
}

// Get the current usage in the counter.
func (u *UsageCounter) Get() jpf.Usage {
	u.lock.Lock()
	defer u.lock.Unlock()
	return u.usage
}

type usageCountingModel struct {
	counter *UsageCounter
	model   jpf.Model
}

func (c *usageCountingModel) Respond(ctx context.Context, messages []jpf.Message) (jpf.ModelResponse, error) {
	resp, err := c.model.Respond(ctx, messages)
	c.counter.Add(resp.Usage)
	return resp, err
}
