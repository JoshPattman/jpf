package jpf

import "errors"

func NewCachedChatCaller(caller ChatCaller, cache Cache) ChatCaller {
	return NewCachedCaller(
		caller,
		cache,
		HashMessages,
		EncodeChatResult,
		DecodeChatResult,
	)
}

func NewCachedEmbedCaller(caller EmbedCaller, cache Cache) EmbedCaller {
	return NewCachedCaller(
		caller,
		cache,
		func(s string) string { return s },
		EncodeEmbedResult,
		DecodeEmbedResult,
	)
}

func NewCachedCaller[T, U any](
	caller Caller[T, U],
	cache Cache,
	encodeKey func(T) string,
	encodeVal func(U) ([]byte, error),
	decodeVal func([]byte) (U, error),
) Caller[T, U] {
	return &cachedCaller[T, U]{caller, cache, encodeKey, encodeVal, decodeVal}
}

type cachedCaller[T, U any] struct {
	caller    Caller[T, U]
	cache     Cache
	encodeKey func(T) string
	encodeVal func(U) ([]byte, error)
	decodeVal func([]byte) (U, error)
}

func (c *cachedCaller[T, U]) Call(inp T) (U, error) {
	var u U
	key := c.encodeKey(inp)
	cachedBytes, err := c.cache.Get(key)
	if errors.Is(err, ErrNoCache) {
		actualResult, err := c.caller.Call(inp)
		if err != nil {
			return u, err
		}
		encodedBytes, err := c.encodeVal(actualResult)
		if err != nil {
			return u, errors.Join(errors.New("failed to encode value to cache"), err)
		}
		err = c.cache.Set(key, encodedBytes)
		if err != nil {
			return u, errors.Join(errors.New("failed to set value in cache"), err)
		}
		return actualResult, nil
	} else if err != nil {
		return u, errors.Join(errors.New("failed to get cached value"), err)
	} else {
		cachedVal, err := c.decodeVal(cachedBytes)
		if err != nil {
			return u, errors.Join(errors.New("failed to decode cached value"), err)
		}
		return cachedVal, nil
	}
}
