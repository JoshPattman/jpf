package jpf

import (
	"fmt"
)

type ConcurrentLimiter chan struct{}

func NewMaxConcurrentLimiter(n int) ConcurrentLimiter {
	return make(ConcurrentLimiter, n)
}

func NewOneConcurrentLimiter() ConcurrentLimiter {
	return NewMaxConcurrentLimiter(1)
}

// Builds a model that has a maximum number of concurrent uses at once.
// The default number of uses is 1.
// There is no certainty about the order of calls (i.e. a later call made to this may be processed before an earlier one).
func BuildConcurrentLimitedModel(builder ModelBuilder, limiter ConcurrentLimiter) *ConcurrentLimitedModelBuilder {
	return &ConcurrentLimitedModelBuilder{
		builder: builder,
		limiter: limiter,
	}
}

type ConcurrentLimitedModelBuilder struct {
	builder ModelBuilder
	limiter ConcurrentLimiter
}

func (m *ConcurrentLimitedModelBuilder) New() (Model, error) {
	if m.limiter == nil {
		return nil, fmt.Errorf("must specify a non-nil limiter")
	}
	if cap(m.limiter) == 0 {
		return nil, fmt.Errorf("must have at least one length limiter, got %d", cap(m.limiter))
	}
	if m.builder == nil {
		return nil, fmt.Errorf("must specify a sub builder")
	}
	subModel, err := m.builder.New()
	if err != nil {
		return nil, err
	}
	return &concurrentLimitedModel{
		model: subModel,
		uses:  m.limiter,
	}, nil
}

type concurrentLimitedModel struct {
	model Model
	uses  ConcurrentLimiter
}

func (c *concurrentLimitedModel) Tokens() (int, int) {
	return c.model.Tokens()
}

func (c *concurrentLimitedModel) Respond(messages []Message) ([]Message, Message, Usage, error) {
	c.uses <- struct{}{}
	defer func() { <-c.uses }()
	return c.model.Respond(messages)
}
