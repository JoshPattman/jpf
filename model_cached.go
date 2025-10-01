package jpf

// NewCachedModel wraps a Model with response caching functionality.
// It stores responses in the provided ModelResponseCache implementation,
// returning cached results for identical input messages to avoid redundant model calls.
func NewCachedModel(model Model, cache ModelResponseCache, opts ...CachedModelOpt) Model {
	m := &cachedModel{
		model: model,
		cache: cache,
	}
	for _, o := range opts {
		o.applyCachedModel(m)
	}
	return m
}

type CachedModelOpt interface {
	applyCachedModel(*cachedModel)
}

func (o WithSalt) applyCachedModel(m *cachedModel) { m.salt = o.X }

type cachedModel struct {
	model Model
	cache ModelResponseCache
	salt  string
}

// Respond implements Model.
func (c *cachedModel) Respond(msgs []Message) (ModelResponse, error) {
	ok, aux, final, err := c.cache.GetCachedResponse(c.salt, msgs)
	if err != nil {
		return ModelResponse{}, wrap(err, "failed to query cache")
	}
	if ok {
		return ModelResponse{
			AuxilliaryMessages: aux,
			PrimaryMessage:     final,
		}, nil
	}
	resp, err := c.model.Respond(msgs)
	if err != nil {
		return resp.OnlyUsage(), err
	}
	err = c.cache.SetCachedResponse(c.salt, msgs, resp.AuxilliaryMessages, resp.PrimaryMessage)
	if err != nil {
		return resp.OnlyUsage(), wrap(err, "failed to set cache")
	}
	return resp, nil
}
