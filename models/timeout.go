package models

import (
	"context"
	"time"

	"github.com/JoshPattman/jpf"
)

// NewTimeoutModel creates a new model that will cause the context of the child to timeout,
// a specified duration after Respond is called.
// Caution: It only tells the context to timeout - it will not forecfully stop the child model if it does not respect the context.
func NewTimeoutModel(model jpf.Model, timeout time.Duration) jpf.Model {
	return &timeoutModel{
		Model:   model,
		timeout: timeout,
	}
}

type timeoutModel struct {
	jpf.Model
	timeout time.Duration
}

func (m *timeoutModel) Respond(ctx context.Context, msgs []jpf.Message) (jpf.ModelResponse, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()
	return m.Model.Respond(timeoutCtx, msgs)
}
