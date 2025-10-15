package jpf

import "context"

// NewInMemoryCache creates an in-memory implementation of ModelResponseCache.
// It stores model responses in memory using a hash of the input messages as a key.
func NewInMemoryCache() ModelResponseCache {
	return &inMemoryCache{
		resps: make(map[string]memoryCachePacket),
	}
}

type memoryCachePacket struct {
	aux   []Message
	final Message
}

type inMemoryCache struct {
	resps map[string]memoryCachePacket
}

// GetCachedResponse implements ModelResponseCache.
func (i *inMemoryCache) GetCachedResponse(ctx context.Context, salt string, msgs []Message) (bool, []Message, Message, error) {
	msgsHash := HashMessages(salt, msgs)
	if cp, ok := i.resps[msgsHash]; ok {
		return true, cp.aux, cp.final, nil
	}
	return false, nil, Message{}, nil
}

// SetCachedResponse implements ModelResponseCache.
func (i *inMemoryCache) SetCachedResponse(ctx context.Context, salt string, inputs []Message, aux []Message, out Message) error {
	msgsHash := HashMessages(salt, inputs)
	i.resps[msgsHash] = memoryCachePacket{
		aux:   aux,
		final: out,
	}
	return nil
}
