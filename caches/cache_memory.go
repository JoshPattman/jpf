package caches

import (
	"context"

	"github.com/JoshPattman/jpf"
)

// NewRAM creates an in-memory implementation of ModelResponseCache.
// It stores model responses in memory using a hash of the input messages as a key.
func NewRAM() jpf.ModelResponseCache {
	return &inMemoryCache{
		Resps: make(map[string]memoryCachePacket),
	}
}

type memoryCachePacket struct {
	Final jpf.Message
}

type inMemoryCache struct {
	Resps map[string]memoryCachePacket
}

// GetCachedResponse implements ModelResponseCache.
func (i *inMemoryCache) GetCachedResponse(ctx context.Context, salt string, msgs []jpf.Message) (bool, jpf.Message, error) {
	msgsHash := HashMessages(salt, msgs)
	if cp, ok := i.Resps[msgsHash]; ok {
		return true, cp.Final, nil
	}
	return false, jpf.Message{}, nil
}

// SetCachedResponse implements ModelResponseCache.
func (i *inMemoryCache) SetCachedResponse(ctx context.Context, salt string, inputs []jpf.Message, out jpf.Message) error {
	msgsHash := HashMessages(salt, inputs)
	i.Resps[msgsHash] = memoryCachePacket{
		Final: out,
	}
	return nil
}
