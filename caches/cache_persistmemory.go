package caches

import (
	"context"
	"encoding/gob"
	"os"
	"sync"

	"github.com/JoshPattman/jpf"
)

// NewFile creates an in-memory cache that persists to the given filename.
// On creation, it loads the cache from the file (if it exists). Whenever SetCachedResponse
// is called, the entire cache is saved back to the file.
func NewFile(filename string) (jpf.ModelResponseCache, error) {
	cache := &filePersistCache{
		resps:    make(map[string]memoryCachePacket),
		filename: filename,
	}
	if err := cache.load(); err != nil {
		return nil, err
	}
	return cache, nil
}

type filePersistCache struct {
	mu       sync.Mutex
	resps    map[string]memoryCachePacket
	filename string
}

func (f *filePersistCache) load() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	file, err := os.Open(f.filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // file doesn't exist yet
		}
		return err
	}
	defer file.Close()

	decoder := gob.NewDecoder(file)
	return decoder.Decode(&f.resps)
}

func (f *filePersistCache) save() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	file, err := os.Create(f.filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := gob.NewEncoder(file)
	return encoder.Encode(f.resps)
}

func (f *filePersistCache) GetCachedResponse(ctx context.Context, salt string, msgs []jpf.Message) (bool, []jpf.Message, jpf.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	msgsHash := HashMessages(salt, msgs)
	if cp, ok := f.resps[msgsHash]; ok {
		return true, cp.Aux, cp.Final, nil
	}
	return false, nil, jpf.Message{}, nil
}

func (f *filePersistCache) SetCachedResponse(ctx context.Context, salt string, inputs []jpf.Message, aux []jpf.Message, out jpf.Message) error {
	f.mu.Lock()
	msgsHash := HashMessages(salt, inputs)
	f.resps[msgsHash] = memoryCachePacket{
		Aux:   aux,
		Final: out,
	}
	f.mu.Unlock()

	return f.save()
}
