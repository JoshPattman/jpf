package caches

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/JoshPattman/jpf"
)

func TestFileCacheMissThenHit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.gob")
	cache, err := NewFile(path)
	if err != nil {
		t.Fatal(err)
	}
	msgs := []jpf.Message{jpf.UserMessage{Content: "hi"}}

	hit, _, err := cache.GetCachedResponse(context.Background(), "salt", msgs)
	if err != nil {
		t.Fatal(err)
	}
	if hit {
		t.Fatal("expected a miss on an empty cache")
	}

	want := jpf.AssistantMessage{Content: "hello"}
	if err := cache.SetCachedResponse(context.Background(), "salt", msgs, want); err != nil {
		t.Fatal(err)
	}

	hit, resp, err := cache.GetCachedResponse(context.Background(), "salt", msgs)
	if err != nil {
		t.Fatal(err)
	}
	if !hit || resp.Content != want.Content {
		t.Fatalf("expected a hit with %+v, got hit=%v resp=%+v", want, hit, resp)
	}
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
