package jpf

/*
// NewConcurrentLimitedModel wraps a Model with concurrency control.
// It ensures that only a limited number of concurrent calls can be made to the underlying model,
// using the provided ConcurrentLimiter to manage access.
func NewConcurrentLimitedModel(model Model, limiter ConcurrentLimiter) Model {
	return &concurrentLimitedModel{
		model: model,
		uses:  limiter,
	}
}

type ConcurrentLimiter chan struct{}

// NewMaxConcurrentLimiter creates a ConcurrentLimiter that allows up to n concurrent operations.
// The limiter is implemented as a buffered channel with capacity n.
func NewMaxConcurrentLimiter(n int) ConcurrentLimiter {
	return make(ConcurrentLimiter, n)
}

// NewOneConcurrentLimiter creates a ConcurrentLimiter that allows only one operation at a time.
// This is a convenience function equivalent to NewMaxConcurrentLimiter(1).
func NewOneConcurrentLimiter() ConcurrentLimiter {
	return NewMaxConcurrentLimiter(1)
}

type concurrentLimitedModel struct {
	model Model
	uses  ConcurrentLimiter
}

func (c *concurrentLimitedModel) Tokens() (int, int) {
	return c.model.Tokens()
}

func (c *concurrentLimitedModel) Respond(messages []Message) (ChatResult, error) {
	c.uses <- struct{}{}
	defer func() { <-c.uses }()
	return c.model.Respond(messages)
}
*/
