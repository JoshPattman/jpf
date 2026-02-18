package models

import (
	"context"

	"github.com/JoshPattman/jpf"
	"github.com/JoshPattman/jpf/internal/utils"
)

// Cache wraps a Model with response caching functionality.
// It stores responses in the provided ModelResponseCache implementation,
// returning cached results for identical input messages and salts to avoid redundant model calls.
func Cache(model jpf.Model, cache jpf.ModelResponseCache, opts ...CachedModelOpt) jpf.Model {
	m := &cachedModel{
		model: model,
		cache: cache,
	}
	for _, o := range opts {
		o(m)
	}
	return m
}

type CachedModelOpt func(*cachedModel)

func WithSalt(salt string) func(m *cachedModel) {
	return func(m *cachedModel) { m.salt = salt }
}

type cachedModel struct {
	model jpf.Model
	cache jpf.ModelResponseCache
	salt  string
}

// Respond implements Model.
func (c *cachedModel) Respond(ctx context.Context, msgs []jpf.Message) (jpf.ModelResponse, error) {
	ok, final, err := c.cache.GetCachedResponse(ctx, c.salt, msgs)
	if err != nil {
		return jpf.ModelResponse{}, utils.Wrap(err, "failed to query cache")
	}
	if ok {
		return jpf.ModelResponse{
			Message: final,
		}, nil
	}
	resp, err := c.model.Respond(ctx, msgs)
	if err != nil {
		return resp.OnlyUsage(), err
	}
	err = c.cache.SetCachedResponse(ctx, c.salt, msgs, resp.Message)
	if err != nil {
		return resp.OnlyUsage(), utils.Wrap(err, "failed to set cache")
	}
	return resp, nil
}
