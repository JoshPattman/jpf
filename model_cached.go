package jpf

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

type ModelResponseCache interface {
	GetCachedResponse([]Message) (bool, []Message, Message, error)
	SetCachedResponse(inputs []Message, aux []Message, out Message) error
}

func HashMessages(msgs []Message) string {
	s := &strings.Builder{}
	s.WriteString("Messages")
	for _, msg := range msgs {
		s.WriteString(msg.Role.String())
		s.WriteString(msg.Content)
	}
	src := s.String()
	hasher := sha256.New()
	hasher.Write([]byte(src))
	hashBytes := hasher.Sum(nil)
	return hex.EncodeToString(hashBytes)
}

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
func (i *inMemoryCache) GetCachedResponse(msgs []Message) (bool, []Message, Message, error) {
	msgsHash := HashMessages(msgs)
	if cp, ok := i.resps[msgsHash]; ok {
		return true, cp.aux, cp.final, nil
	}
	return false, nil, Message{}, nil
}

// SetCachedResponse implements ModelResponseCache.
func (i *inMemoryCache) SetCachedResponse(inputs []Message, aux []Message, out Message) error {
	msgsHash := HashMessages(inputs)
	i.resps[msgsHash] = memoryCachePacket{
		aux:   aux,
		final: out,
	}
	return nil
}

type CachedModelBuilder struct {
	builder ModelBuilder
	cache   ModelResponseCache
}

func BuildCachedModel(builder ModelBuilder, cache ModelResponseCache) *CachedModelBuilder {
	return &CachedModelBuilder{
		builder: builder,
		cache:   cache,
	}
}

func (b *CachedModelBuilder) New() (Model, error) {
	if b.cache == nil {
		return nil, fmt.Errorf("cannot have a nil cache")
	}
	if b.builder == nil {
		return nil, fmt.Errorf("cannot have a nil base model")
	}
	subModel, err := b.builder.New()
	if err != nil {
		return nil, err
	}
	return &cachedModel{model: subModel, cache: b.cache}, nil
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
	return c.model.Respond(msgs)
}

// Tokens implements Model.
func (c *cachedModel) Tokens() (int, int) {
	return c.model.Tokens()
}
