package models

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/JoshPattman/jpf"
	"github.com/JoshPattman/jpf/utils"
)

type ModelResponseCache interface {
	GetCachedResponse(ctx context.Context, salt string, inputs []jpf.Message) (bool, []jpf.Message, jpf.Message, error)
	SetCachedResponse(ctx context.Context, salt string, inputs []jpf.Message, aux []jpf.Message, out jpf.Message) error
}

// Cache wraps a Model with response caching functionality.
// It stores responses in the provided ModelResponseCache implementation,
// returning cached results for identical input messages and salts to avoid redundant model calls.
func Cache(model jpf.Model, cache ModelResponseCache, opts ...CachedModelOpt) jpf.Model {
	m := &cachedModel{
		model: model,
		cache: cache,
	}
	for _, o := range opts {
		o(m)
	}
	return m
}

type CachedModelOpt func(*cachedModel)

func WithSalt(salt string) func(m *cachedModel) {
	return func(m *cachedModel) { m.salt = salt }
}

type cachedModel struct {
	model jpf.Model
	cache ModelResponseCache
	salt  string
}

// Respond implements Model.
func (c *cachedModel) Respond(ctx context.Context, msgs []jpf.Message) (jpf.ModelResponse, error) {
	ok, aux, final, err := c.cache.GetCachedResponse(ctx, c.salt, msgs)
	if err != nil {
		return jpf.ModelResponse{}, utils.Wrap(err, "failed to query cache")
	}
	if ok {
		return jpf.ModelResponse{
			AuxiliaryMessages: aux,
			PrimaryMessage:    final,
		}, nil
	}
	resp, err := c.model.Respond(ctx, msgs)
	if err != nil {
		return resp.OnlyUsage(), err
	}
	err = c.cache.SetCachedResponse(ctx, c.salt, msgs, resp.AuxiliaryMessages, resp.PrimaryMessage)
	if err != nil {
		return resp.OnlyUsage(), utils.Wrap(err, "failed to set cache")
	}
	return resp, nil
}

func HashMessages(salt string, inputs []jpf.Message) string {
	s := &strings.Builder{}
	s.WriteString(salt)
	s.WriteString("Messages")
	for _, msg := range inputs {
		s.WriteString(msg.Role.String())
		s.WriteString(msg.Content)
		for _, img := range msg.Images {
			imgString, err := img.ToBase64Encoded(false)
			if err != nil {
				panic(err)
			}
			s.WriteString(imgString)
		}
	}
	src := s.String()
	hasher := sha256.New()
	hasher.Write([]byte(src))
	hashBytes := hasher.Sum(nil)
	return hex.EncodeToString(hashBytes)
}
