package jpf

// NewLimitConcurrencyCaller wraps a Model with concurrency control.
// It ensures that only a limited number of concurrent calls can be made to the underlying model,
// using the provided ConcurrentLimiter to manage access.
func NewLimitConcurrencyCaller[T, U any](model Caller[T, U], limiter ConcurrentLimiter) Caller[T, U] {
	return &limitConcurrencyCaller[T, U]{
		model: model,
		uses:  limiter,
	}
}

type ConcurrentLimiter chan struct{}

// MaxConcurrency creates a ConcurrentLimiter that allows up to n concurrent operations.
// The limiter is implemented as a buffered channel with capacity n.
func MaxConcurrency(n int) ConcurrentLimiter {
	return make(ConcurrentLimiter, n)
}

// OneConcurrency creates a ConcurrentLimiter that allows only one operation at a time.
// This is a convenience function equivalent to NewMaxConcurrentLimiter(1).
func OneConcurrency() ConcurrentLimiter {
	return MaxConcurrency(1)
}

type limitConcurrencyCaller[T, U any] struct {
	model Caller[T, U]
	uses  ConcurrentLimiter
}

func (c *limitConcurrencyCaller[T, U]) Call(input T) (U, error) {
	c.uses <- struct{}{}
	defer func() { <-c.uses }()
	return c.model.Call(input)
}
