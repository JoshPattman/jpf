package jpf

import (
	"context"
	"errors"

	"golang.org/x/time/rate"
)

var _ Model = &rateLimitedModel{}

func NewRateLimitedModel(model Model, limiter *rate.Limiter) Model {
	return &rateLimitedModel{
		limiter: limiter,
		model:   model,
	}
}

type rateLimitedModel struct {
	limiter *rate.Limiter
	model   Model
}

func (r *rateLimitedModel) Respond(ctx context.Context, msgs []Message) (ModelResponse, error) {
	err := r.limiter.Wait(ctx)
	if err != nil {
		return ModelResponse{}, errors.Join(errors.New("failed to wait for rate limiter"), err)
	}
	return r.model.Respond(ctx, msgs)
}
