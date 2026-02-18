package models

import (
	"context"

	"github.com/JoshPattman/jpf"
	"golang.org/x/sync/semaphore"
)

// LimitConcurrency wraps a Model with concurrency control.
// It ensures that only a limited number of concurrent calls can be made to the underlying model,
// using the provided semaphore to manage access (this can be shared between different model instances to ensure control at any level).
func LimitConcurrency(model jpf.Model, limiter *semaphore.Weighted) jpf.Model {
	return &concurrentLimitedModel{
		model: model,
		sem:   limiter,
	}
}

type concurrentLimitedModel struct {
	model jpf.Model
	sem   *semaphore.Weighted
}

func (c *concurrentLimitedModel) Respond(ctx context.Context, messages []jpf.Message) (jpf.ModelResponse, error) {
	err := c.sem.Acquire(ctx, 1)
	if err != nil {
		return jpf.ModelResponse{}, err
	}
	defer c.sem.Release(1)
	return c.model.Respond(ctx, messages)
}
