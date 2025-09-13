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
		return ModelResponse{}, err
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
		return resp.OnlyUsage(), err
	}
	return resp, nil
}

// Tokens implements Model.
func (c *cachedModel) Tokens() (int, int) {
	return c.model.Tokens()
}
