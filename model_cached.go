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
func (c *cachedModel) Respond(msgs []Message) (ChatResult, error) {
	ok, aux, final, err := c.cache.GetCachedResponse(msgs)
	if err != nil {
		return ChatResult{}, err
	}
	if ok {
		return ChatResult{Extra: aux, Primary: final}, nil
	}
	res, err := c.model.Respond(msgs)
	if err != nil {
		return res.OnlyUsage(), err
	}
	err = c.cache.SetCachedResponse(msgs, res.Extra, res.Primary)
	if err != nil {
		return res.OnlyUsage(), err
	}
	return res, nil
}

// Tokens implements Model.
func (c *cachedModel) Tokens() (int, int) {
	return c.model.Tokens()
}
