package jpf

import (
	"sync"
)

// NewUsageCountingModel wraps a Model with token usage tracking functionality.
// It aggregates token usage statistics in the provided UsageCounter,
// which allows monitoring total token consumption across multiple model calls.
func NewUsageCountingModel(model Model, counter *UsageCounter) Model {
	return &usageCountingModel{
		counter: counter,
		model:   model,
	}
}

// Counts up the sum usage.
// Is completely concurrent-safe.
type UsageCounter struct {
	usage Usage
	lock  *sync.Mutex
}

// NewUsageCounter creates a new UsageCounter with zero initial usage.
// The counter is safe for concurrent use across multiple goroutines.
func NewUsageCounter() *UsageCounter {
	return &UsageCounter{
		usage: Usage{},
		lock:  &sync.Mutex{},
	}
}

// Add the given usage to the counter.
func (u *UsageCounter) Add(usage Usage) {
	u.lock.Lock()
	defer u.lock.Unlock()
	u.usage = u.usage.Add(usage)
}

// Get the current usage in the counter.
func (u *UsageCounter) Get() Usage {
	u.lock.Lock()
	defer u.lock.Unlock()
	return u.usage
}

type usageCountingModel struct {
	counter *UsageCounter
	model   Model
}

func (c *usageCountingModel) Tokens() (int, int) {
	return c.model.Tokens()
}

func (c *usageCountingModel) Respond(messages []Message) ([]Message, Message, Usage, error) {
	auxMessages, response, usage, err := c.model.Respond(messages)
	c.counter.Add(usage)
	return auxMessages, response, usage, err
}
