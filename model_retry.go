package jpf

import (
	"time"
)

// NewRetryModel wraps a Model with retry functionality.
// If the underlying model returns an error, this wrapper will retry the operation
// up to a configurable number of times with an optional delay between retries.
func NewRetryModel[T, U any](model Caller[T, U], opts ...retryModelOpt) Caller[T, U] {
	m := &retryModel[T, U]{
		Caller:  model,
		retries: 99999,
		delay:   0,
	}
	for _, o := range opts {
		o.applyRetry(m)
	}
	return m
}

type retryModelOpt interface {
	applyRetry(*retryModel)
}

func (o WithRetries) applyRetry(m *retryModel[T, U]) { m.retries = o.X }
func (o WithDelay) applyRetry(m *retryModel)         { m.delay = o.X }

type retryModel[T, U any] struct {
	Caller  Caller[T, U]
	retries int
	delay   time.Duration
}

func (m *retryModel[T, U]) Call(inp T) (U, error) {
	var res U
	var err error
	for range m.retries + 1 {
		res, err = m.Caller.Call(inp)
		if err == nil {
			break
		}
		time.Sleep(m.delay)
	}
	if err != nil {
		return res, err
	}
	return res, nil
}
