package jpf

import (
	"time"
)

// NewRetryModel wraps a Model with retry functionality.
// If the underlying model returns an error, this wrapper will retry the operation
// up to a configurable number of times with an optional delay between retries.
func NewRetryModel(model Model, maxRetries int, opts ...RetryModelOpt) Model {
	m := &retryModel{
		Model:   model,
		retries: maxRetries,
		delay:   0,
	}
	for _, o := range opts {
		o.applyRetry(m)
	}
	return m
}

type RetryModelOpt interface {
	applyRetry(*retryModel)
}

func (o WithDelay) applyRetry(m *retryModel) { m.delay = o.X }

type retryModel struct {
	Model
	retries int
	delay   time.Duration
}

func (m *retryModel) Respond(msgs []Message) (ModelResponse, error) {
	var totalUsageSoFar Usage
	var err error
	for range m.retries + 1 {
		var resp ModelResponse
		resp, err = m.Model.Respond(msgs)
		resp = resp.IncludingUsage(totalUsageSoFar)
		if err == nil {
			return resp, nil
		}
		totalUsageSoFar = resp.Usage
		time.Sleep(m.delay)
	}
	return ModelResponse{Usage: totalUsageSoFar}, wrap(err, "could not get model response after retrying %d times", m.retries)
}
