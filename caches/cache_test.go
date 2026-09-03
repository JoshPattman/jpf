package caches

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/JoshPattman/jpf"
	"github.com/JoshPattman/jpf/internal/utils"
)

type CacheCase struct {
	ID    string
	Build func() jpf.ModelResponseCache
}

func (c CacheCase) Name() string { return c.ID }

func (c CacheCase) Test() error {
	cache := c.Build()
	msgs := []jpf.Message{jpf.UserMessage{Content: "hi"}}

	hit, resp, err := cache.GetCachedResponse(context.Background(), "salt", msgs)
	if err != nil {
		return err
	}
	if hit {
		return fmt.Errorf("expected a miss, got %+v", resp)
	}

	want := jpf.AssistantMessage{Content: "hello"}
	if err := cache.SetCachedResponse(context.Background(), "salt", msgs, want); err != nil {
		return err
	}

	hit, resp, err = cache.GetCachedResponse(context.Background(), "salt", msgs)
	if err != nil {
		return err
	}
	if !hit || resp.Content != want.Content {
		return fmt.Errorf("expected a hit with %+v, got hit=%v resp=%+v", want, hit, resp)
	}

	hit, _, err = cache.GetCachedResponse(context.Background(), "other-salt", msgs)
	if err != nil {
		return err
	}
	if hit {
		return errors.New("expected a different salt to miss")
	}
	return nil
}

func newTempFileCache() jpf.ModelResponseCache {
	path := filepath.Join(os.TempDir(), fmt.Sprintf("jpf-cache-test-%d.gob", time.Now().UnixNano()))
	cache, err := NewFile(path)
	if err != nil {
		panic(err)
	}
	return cache
}

var CacheCases = []utils.TestCase{
	CacheCase{ID: "memory", Build: func() jpf.ModelResponseCache { return NewRAM() }},
	CacheCase{ID: "file", Build: newTempFileCache},
}

func TestCache(t *testing.T) {
	utils.RunTests(t, CacheCases)
}

func TestFileCachePersistsAcrossInstances(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.gob")
	msgs := []jpf.Message{jpf.UserMessage{Content: "hi"}}

	cache, err := NewFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := cache.SetCachedResponse(context.Background(), "salt", msgs, jpf.AssistantMessage{Content: "hello"}); err != nil {
		t.Fatal(err)
	}

	reloaded, err := NewFile(path)
	if err != nil {
		t.Fatal(err)
	}
	hit, resp, err := reloaded.GetCachedResponse(context.Background(), "salt", msgs)
	if err != nil {
		t.Fatal(err)
	}
	if !hit || resp.Content != "hello" {
		t.Fatalf("expected the reloaded cache to have the saved entry, got hit=%v resp=%+v", hit, resp)
	}
}

func TestFileCacheLoadErrorsOnCorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.gob")
	if err := os.WriteFile(path, []byte("not a gob file"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewFile(path); err == nil {
		t.Fatal("expected an error loading a corrupt cache file")
	}
}
