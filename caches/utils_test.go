package caches

import (
	"image"
	"testing"

	"github.com/JoshPattman/jpf"
)

func TestHashMessagesIsDeterministic(t *testing.T) {
	msgs := []jpf.Message{jpf.UserMessage{Content: "hi"}}
	a := HashMessages("salt", msgs)
	b := HashMessages("salt", msgs)
	if a != b {
		t.Fatalf("expected the same hash, got %s and %s", a, b)
	}
}

func TestHashMessagesDiffersBySalt(t *testing.T) {
	msgs := []jpf.Message{jpf.UserMessage{Content: "hi"}}
	a := HashMessages("salt-a", msgs)
	b := HashMessages("salt-b", msgs)
	if a == b {
		t.Fatal("expected different salts to produce different hashes")
	}
}

func TestHashMessagesDiffersByContent(t *testing.T) {
	a := HashMessages("salt", []jpf.Message{jpf.UserMessage{Content: "hi"}})
	b := HashMessages("salt", []jpf.Message{jpf.UserMessage{Content: "bye"}})
	if a == b {
		t.Fatal("expected different content to produce different hashes")
	}
}

func TestMessageToStringCoversAllTypes(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	tests := []jpf.Message{
		jpf.UserMessage{Content: "hi"},
		jpf.UserMessage{Content: "hi", Images: []jpf.ImageAttachment{{Source: img}}},
		jpf.AssistantMessage{Content: "hi", ToolCalls: []jpf.ToolCall{{ID: "c1"}}},
		jpf.DeveloperMessage{Content: "hi"},
		jpf.SystemMessage{Content: "hi"},
		jpf.ToolResultMessage{CallID: "c1", Result: "done"},
	}
	seen := map[string]bool{}
	for _, msg := range tests {
		s := messageToString(msg)
		if s == "" {
			t.Fatalf("expected a non-empty string for %T", msg)
		}
		if seen[s] {
			t.Fatalf("expected a unique string for %T, got a duplicate: %s", msg, s)
		}
		seen[s] = true
	}
}

func TestMessageToStringPanicsOnUnknownType(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic for an unknown message type")
		}
	}()
	messageToString(nil)
}
