package jpf

import (
	"time"
)

// NewRetryModel wraps a Model with retry functionality.
// If the underlying model returns an error, this wrapper will retry the operation
// up to a configurable number of times with an optional delay between retries.
func NewRetryModel(model Model, opts ...retryModelOpt) Model {
	m := &retryModel{
		Model:   model,
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

func (o WithRetries) applyRetry(m *retryModel) { m.retries = o.X }
func (o WithDelay) applyRetry(m *retryModel)   { m.delay = o.X }

type retryModel struct {
	Model
	retries int
	delay   time.Duration
}

func (m *retryModel) Respond(msgs []Message) (ModelResult, error) {
	var res ModelResult
	var usgTotal Usage
	var err error
	for range m.retries + 1 {
		res, err = m.Model.Respond(msgs)
		usgTotal = usgTotal.Add(res.Usage)
		if err == nil {
			break
		}
		time.Sleep(m.delay)
	}
	if err != nil {
		return res.OnlyUsage(), err
	}
	return res, nil
}
