package jpf

import "time"

type RetryModelBuilder struct {
	model *retryModel
}

func BuildRetryModel(model Model) *RetryModelBuilder {
	return &RetryModelBuilder{
		model: &retryModel{
			Model: model,
			tries: 99999,
			delay: 0,
		},
	}
}

func (b *RetryModelBuilder) Validate() (Model, error) {
	return b.model, nil
}

func (b *RetryModelBuilder) WithMaxRetries(maxRetries int) *RetryModelBuilder {
	b.model.tries = maxRetries
	return b
}

func (b *RetryModelBuilder) WithDelay(delay time.Duration) *RetryModelBuilder {
	b.model.delay = delay
	return b
}

type retryModel struct {
	Model
	tries int
	delay time.Duration
}

func (m *retryModel) Respond(msgs []Message) ([]Message, Message, Usage, error) {
	var aux []Message
	var msg Message
	var usgTotal Usage
	var usg Usage
	var err error
	for range m.tries {
		aux, msg, usg, err = m.Model.Respond(msgs)
		usgTotal = usgTotal.Add(usg)
		if err == nil {
			break
		}
		time.Sleep(m.delay)
	}
	return aux, msg, usgTotal, err
}
