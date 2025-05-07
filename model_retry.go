package jpf

import "time"

type retryModel struct {
	Model
	tries int
	delay time.Duration
}

func NewRetryModel(model Model, tries int, delay time.Duration) Model {
	return &retryModel{
		Model: model,
		tries: tries,
		delay: delay,
	}
}

func (m *retryModel) Respond(msgs []Message) (Message, Usage, error) {
	var msg Message
	var usg Usage
	var err error
	for range m.tries {
		msg, usg, err = m.Model.Respond(msgs)
		if err == nil {
			break
		}
		time.Sleep(m.delay)
	}
	return msg, usg, err
}
