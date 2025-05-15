package jpf

import "time"

type retryModel struct {
	Model
	tries int
	delay time.Duration
}

// NewRetryModel wraps a model such that it will retry calling it if an error occurs,
// with intermediate delays.
func NewRetryModel(model Model, tries int, delay time.Duration) Model {
	return &retryModel{
		Model: model,
		tries: tries,
		delay: delay,
	}
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
