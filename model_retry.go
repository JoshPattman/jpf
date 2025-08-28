package jpf

import (
	"time"
)

// NewRetryModel wraps a Model with retry functionality.
// If the underlying model returns an error, this wrapper will retry the operation
// up to a configurable number of times with an optional delay between retries.
func NewRetryModel(model Model, opts ...retryModelOpt) Model {
	m := &retryModel{
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

func (m *retryModel) Respond(msgs []Message) ([]Message, Message, Usage, error) {
	var aux []Message
	var msg Message
	var usgTotal Usage
	var usg Usage
	var err error
	for range m.retries + 1 {
		aux, msg, usg, err = m.Model.Respond(msgs)
		usgTotal = usgTotal.Add(usg)
		if err == nil {
			break
		}
		time.Sleep(m.delay)
	}
	return aux, msg, usgTotal, err
}
