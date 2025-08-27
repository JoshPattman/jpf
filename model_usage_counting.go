package jpf

import (
	"fmt"
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

// Builds a model that adds all usage of the child model to the counter.
func BuildUsageCountingModel(builder ModelBuilder, counter *UsageCounter) *UsageCountingModelBuilder {
	return &UsageCountingModelBuilder{
		builder: builder,
		counter: counter,
	}
}

type UsageCountingModelBuilder struct {
	builder ModelBuilder
	counter *UsageCounter
}

func (m *UsageCountingModelBuilder) New() (Model, error) {
	if m.counter == nil {
		return nil, fmt.Errorf("may not have a nil usage counter")
	}
	if m.builder == nil {
		return nil, fmt.Errorf("may not have a nil builder")
	}
	subModel, err := m.builder.New()
	if err != nil {
		return nil, err
	}
	return &usageCountingModel{
		model:   subModel,
		counter: m.counter,
	}, nil
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
