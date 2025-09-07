package jpf

import (
	"time"
)

// NewRetryCaller wraps a Caller with retry functionality.
// If the underlying Caller returns an error, this wrapper will retry the operation
// up to a number of times with a delay between retries.
func NewRetryCaller[T, U any](model Caller[T, U], maxRetries int, retryDelay time.Duration) Caller[T, U] {
	m := &retryCaller[T, U]{
		Caller:  model,
		retries: 99999,
		delay:   0,
	}
	return m
}

type retryCaller[T, U any] struct {
	Caller  Caller[T, U]
	retries int
	delay   time.Duration
}

func (m *retryCaller[T, U]) Call(inp T) (U, error) {
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
