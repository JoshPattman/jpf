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
func (c *cachedModel) Respond(msgs []Message) ([]Message, Message, Usage, error) {
	ok, aux, final, err := c.cache.GetCachedResponse(msgs)
	if err != nil {
		return nil, Message{}, Usage{}, err
	}
	if ok {
		return aux, final, Usage{}, nil
	}
	aux, resp, usage, err := c.model.Respond(msgs)
	if err != nil {
		return nil, Message{}, usage, err
	}
	err = c.cache.SetCachedResponse(msgs, aux, resp)
	if err != nil {
		return nil, Message{}, usage, err
	}
	return aux, resp, usage, nil
}

// Tokens implements Model.
func (c *cachedModel) Tokens() (int, int) {
	return c.model.Tokens()
}
