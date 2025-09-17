package jpf

// NewCachedModel wraps a Model with response caching functionality.
// It stores responses in the provided ModelResponseCache implementation,
// returning cached results for identical input messages to avoid redundant model calls.
func NewCachedModel(model Model, cache ModelResponseCache) Model {
	return &cachedModel{
		model: model,
		cache: cache,
	}
}

type cachedModel struct {
	model Model
	cache ModelResponseCache
}

// Respond implements Model.
func (c *cachedModel) Respond(msgs []Message) (ModelResponse, error) {
	ok, aux, final, err := c.cache.GetCachedResponse(msgs)
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
	err = c.cache.SetCachedResponse(msgs, resp.AuxilliaryMessages, resp.PrimaryMessage)
	if err != nil {
		return resp.OnlyUsage(), wrap(err, "failed to set cache")
	}
	return resp, nil
}
