package jpf

import (
	"sync"
)

// Counts up the sum usage.
// Is completely concurrent-safe.
type UsageCounter struct {
	usage Usage
	lock  *sync.Mutex
}

// Create a zero usage counter.
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

// Builds a model that adds all usage of the child model to the counter.
func BuildUsageCountingModel(model Model, counter *UsageCounter) *UsageCountingModelBuilder {
	return &UsageCountingModelBuilder{
		model: &usageCountingModel{
			model:   model,
			counter: counter,
		},
	}
}

type UsageCountingModelBuilder struct {
	model *usageCountingModel
}

func (m *UsageCountingModelBuilder) Validate() (Model, error) {
	return m.model, nil
}

func (c *usageCountingModel) Tokens() (int, int) {
	return c.model.Tokens()
}

func (c *usageCountingModel) Respond(messages []Message) ([]Message, Message, Usage, error) {
	auxMessages, response, usage, err := c.model.Respond(messages)
	c.counter.Add(usage)
	return auxMessages, response, usage, err
}
