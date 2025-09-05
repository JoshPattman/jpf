package jpf

func NewCachedEmbedder(emb Embedder, cache EmbedderResponseCache) Embedder {
	return &cachedEmbeddingModel{
		cache:    cache,
		fallback: emb,
	}
}

type cachedEmbeddingModel struct {
	cache    EmbedderResponseCache
	fallback Embedder
}

func (c *cachedEmbeddingModel) Embed(text string) ([]float64, error) {
	ok, emb, err := c.cache.GetCachedEmbedding(text)
	if err != nil {
		return nil, err
	}
	if ok {
		return emb, nil
	}
	emb, err = c.fallback.Embed(text)
	if err != nil {
		return nil, err
	}
	err = c.cache.SetCachedEmbedding(text, emb)
	if err != nil {
		return nil, err
	}
	return emb, nil
}
