package caches

import (
	"context"
	"testing"

	"github.com/JoshPattman/jpf"
)

func TestInMemoryCacheMissThenHit(t *testing.T) {
	cache := NewRAM()
	msgs := []jpf.Message{jpf.UserMessage{Content: "hi"}}

	hit, resp, err := cache.GetCachedResponse(context.Background(), "salt", msgs)
	if err != nil {
		t.Fatal(err)
	}
	if hit {
		t.Fatalf("expected a miss, got %+v", resp)
	}

	want := jpf.AssistantMessage{Content: "hello"}
	if err := cache.SetCachedResponse(context.Background(), "salt", msgs, want); err != nil {
		t.Fatal(err)
	}

	hit, resp, err = cache.GetCachedResponse(context.Background(), "salt", msgs)
	if err != nil {
		t.Fatal(err)
	}
	if !hit || resp.Content != want.Content {
		t.Fatalf("expected a hit with %+v, got hit=%v resp=%+v", want, hit, resp)
	}
}

func TestInMemoryCacheDifferentSaltMisses(t *testing.T) {
	cache := NewRAM()
	msgs := []jpf.Message{jpf.UserMessage{Content: "hi"}}
	if err := cache.SetCachedResponse(context.Background(), "salt-a", msgs, jpf.AssistantMessage{Content: "hello"}); err != nil {
		t.Fatal(err)
	}

	hit, _, err := cache.GetCachedResponse(context.Background(), "salt-b", msgs)
	if err != nil {
		t.Fatal(err)
	}
	if hit {
		t.Fatal("expected a different salt to miss")
	}
}
