package models

import (
	"context"

	"github.com/JoshPattman/jpf"
)

func Map(model jpf.Model, f func(jpf.Message) jpf.Message) jpf.Model {
	return &mappingModel{model, f}
}

type mappingModel struct {
	model jpf.Model
	f     func(jpf.Message) jpf.Message
}

func (m *mappingModel) Respond(ctx context.Context, msgs []jpf.Message) (jpf.ModelResponse, error) {
	mappedMessages := make([]jpf.Message, len(msgs))
	for i, msg := range msgs {
		mappedMessages[i] = m.f(msg)
	}
	return m.model.Respond(ctx, mappedMessages)
}
