package jpf

import (
	"fmt"
	"time"
)

type RetryModelBuilder struct {
	builder ModelBuilder
	retries int
	delay   time.Duration
}

func BuildRetryModel(builder ModelBuilder) *RetryModelBuilder {
	return &RetryModelBuilder{
		builder: builder,
		retries: 99999,
		delay:   0,
	}
}

func (b *RetryModelBuilder) New() (Model, error) {
	if b.builder == nil {
		return nil, fmt.Errorf("must have a non-nil builder")
	}
	if b.retries < 0 {
		return nil, fmt.Errorf("cannot have negative retries")
	}
	if b.delay < 0 {
		return nil, fmt.Errorf("cannot have negative delay")
	}
	subModel, err := b.builder.New()
	if err != nil {
		return nil, err
	}
	return &retryModel{
		Model:   subModel,
		retries: b.retries,
		delay:   b.delay,
	}, nil
}

func (b *RetryModelBuilder) WithMaxRetries(maxRetries int) *RetryModelBuilder {
	b.retries = maxRetries
	return b
}

func (b *RetryModelBuilder) WithDelay(delay time.Duration) *RetryModelBuilder {
	b.delay = delay
	return b
}

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
