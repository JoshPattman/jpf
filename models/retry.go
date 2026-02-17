package models

import (
	"context"
	"time"

	"github.com/JoshPattman/jpf"
	"github.com/JoshPattman/jpf/utils"
)

// Retry wraps a Model with retry functionality.
// If the underlying model returns an error, this wrapper will retry the operation
// up to a configurable number of times with an optional delay between retries.
func Retry(model jpf.Model, maxRetries int, opts ...RetryOpt) jpf.Model {
	m := &retryModel{
		Model:   model,
		retries: maxRetries,
		delay:   0,
	}
	for _, o := range opts {
		o(m)
	}
	return m
}

type RetryOpt func(*retryModel)

func WithDelay(delay time.Duration) func(*retryModel) {
	return func(rm *retryModel) { rm.delay = delay }
}

type retryModel struct {
	jpf.Model
	retries int
	delay   time.Duration
}

func (m *retryModel) Respond(ctx context.Context, msgs []jpf.Message) (jpf.ModelResponse, error) {
	var totalUsageSoFar jpf.Usage
	var err error
	for range m.retries + 1 {
		var resp jpf.ModelResponse
		resp, err = m.Model.Respond(ctx, msgs)
		resp = resp.IncludingUsage(totalUsageSoFar)
		if err == nil {
			return resp, nil
		}
		totalUsageSoFar = resp.Usage
		time.Sleep(m.delay)
	}
	return jpf.ModelResponse{Usage: totalUsageSoFar}, utils.Wrap(err, "could not get model response after retrying %d times", m.retries)
}
