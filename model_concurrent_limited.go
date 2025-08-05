package jpf

import (
	"fmt"
)

// Builds a model that has a maximum number of concurrent uses at once.
// The default number of uses is 1.
// There is no certainty about the order of calls (i.e. a later call made to this may be processed before an earlier one).
func BuildConcurrentLimitedModel(model Model) *ConcurrentLimitedModelBuilder {
	return &ConcurrentLimitedModelBuilder{
		subModel: model,
		uses:     1,
	}
}

type ConcurrentLimitedModelBuilder struct {
	subModel Model
	uses     int
}

func (m *ConcurrentLimitedModelBuilder) Validate() (Model, error) {
	if m.uses <= 0 {
		return nil, fmt.Errorf("must have at least one use, got %d", m.uses)
	}
	if m.subModel == nil {
		return nil, fmt.Errorf("must specify a sub model")
	}
	return &concurrentLimitedModel{
		model: m.subModel,
		uses:  make(chan struct{}, m.uses),
	}, nil
}

// Sets the number on concurrent uses, must be >= 1.
func (m *ConcurrentLimitedModelBuilder) WithUses(n int) *ConcurrentLimitedModelBuilder {
	m.uses = n
	return m
}

type concurrentLimitedModel struct {
	model Model
	uses  chan struct{}
}

func (c *concurrentLimitedModel) Tokens() (int, int) {
	return c.model.Tokens()
}

func (c *concurrentLimitedModel) Respond(messages []Message) ([]Message, Message, Usage, error) {
	c.uses <- struct{}{}
	defer func() { <-c.uses }()
	return c.model.Respond(messages)
}
