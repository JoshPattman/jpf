package jpf

import "context"

// NewInMemoryCache creates an in-memory implementation of ModelResponseCache.
// It stores model responses in memory using a hash of the input messages as a key.
func NewInMemoryCache() ModelResponseCache {
	return &inMemoryCache{
		Resps: make(map[string]memoryCachePacket),
	}
}

type memoryCachePacket struct {
	Aux   []Message
	Final Message
}

type inMemoryCache struct {
	Resps map[string]memoryCachePacket
}

// GetCachedResponse implements ModelResponseCache.
func (i *inMemoryCache) GetCachedResponse(ctx context.Context, salt string, msgs []Message) (bool, []Message, Message, error) {
	msgsHash := HashMessages(salt, msgs)
	if cp, ok := i.Resps[msgsHash]; ok {
		return true, cp.Aux, cp.Final, nil
	}
	return false, nil, Message{}, nil
}

// SetCachedResponse implements ModelResponseCache.
func (i *inMemoryCache) SetCachedResponse(ctx context.Context, salt string, inputs []Message, aux []Message, out Message) error {
	msgsHash := HashMessages(salt, inputs)
	i.Resps[msgsHash] = memoryCachePacket{
		Aux:   aux,
		Final: out,
	}
	return nil
}
