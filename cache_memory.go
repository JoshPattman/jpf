package jpf

import (
	"fmt"
)

// NewInMemoryCache creates an in-memory implementation of ModelResponseCache.
// It stores model responses in memory using a hash of the input messages as a key.
func NewInMemoryCache() Cache {
	return &inMemoryCache{
		resps: make(map[string]memoryCachePacket),
		embs:  make(map[string][]float64),
	}
}

type memoryCachePacket struct {
	aux   []Message
	final Message
}

type inMemoryCache struct {
	resps map[string]memoryCachePacket
	embs  map[string][]float64
}

// GetCachedResponse implements ModelResponseCache.
func (i *inMemoryCache) GetCachedResponse(msgs []Message) (bool, []Message, Message, error) {
	msgsHash := HashMessages(msgs)
	fmt.Println("GET", msgsHash)
	if cp, ok := i.resps[msgsHash]; ok {
		return true, cp.aux, cp.final, nil
	}
	return false, nil, Message{}, nil
}

// SetCachedResponse implements ModelResponseCache.
func (i *inMemoryCache) SetCachedResponse(inputs []Message, aux []Message, out Message) error {
	msgsHash := HashMessages(inputs)
	fmt.Println("SET", msgsHash)
	i.resps[msgsHash] = memoryCachePacket{
		aux:   aux,
		final: out,
	}
	return nil
}

func (cache *inMemoryCache) GetCachedEmbedding(s string) (bool, []float64, error) {
	if emb, ok := cache.embs[s]; ok {
		return true, emb, nil
	}
	return false, nil, nil
}

func (cache *inMemoryCache) SetCachedEmbedding(s string, e []float64) error {
	cache.embs[s] = e
	return nil
}
