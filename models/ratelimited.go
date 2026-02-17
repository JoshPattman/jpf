package models

import (
	"context"
	"errors"

	"github.com/JoshPattman/jpf"
	"golang.org/x/time/rate"
)

func RateLimit(model jpf.Model, limiter *rate.Limiter) jpf.Model {
	return &rateLimitedModel{
		limiter: limiter,
		model:   model,
	}
}

type rateLimitedModel struct {
	limiter *rate.Limiter
	model   jpf.Model
}

func (r *rateLimitedModel) Respond(ctx context.Context, msgs []jpf.Message) (jpf.ModelResponse, error) {
	err := r.limiter.Wait(ctx)
	if err != nil {
		return jpf.ModelResponse{}, errors.Join(errors.New("failed to wait for rate limiter"), err)
	}
	return r.model.Respond(ctx, msgs)
}
