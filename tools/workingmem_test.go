package tools

import (
	"context"
	"testing"

	"github.com/JoshPattman/jpf"
)

func TestUpdateMemoryFillsEmptyMemory(t *testing.T) {
	tools, cbs := BuildWorkingMemoryUtils()
	update := tools[0]

	_, err := update.Call(context.Background(), jpf.ToolArgs{"old_text": "", "new_text": "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	frags := cbs[0](jpf.AgentSession{})
	if frags["working_memory"] != "hello" {
		t.Fatalf("got %q", frags["working_memory"])
	}
}

func TestUpdateMemoryFailsToFillNonEmptyMemory(t *testing.T) {
	tools, _ := BuildWorkingMemoryUtils()
	update := tools[0]

	if _, err := update.Call(context.Background(), jpf.ToolArgs{"old_text": "", "new_text": "hello"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := update.Call(context.Background(), jpf.ToolArgs{"old_text": "", "new_text": "world"}); err == nil {
		t.Fatalf("expected an error filling already-non-empty memory")
	}
}

func TestUpdateMemoryReplacesUniqueMatch(t *testing.T) {
	tools, cbs := BuildWorkingMemoryUtils()
	update := tools[0]

	if _, err := update.Call(context.Background(), jpf.ToolArgs{"old_text": "", "new_text": "the cat sat"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := update.Call(context.Background(), jpf.ToolArgs{"old_text": "cat", "new_text": "dog"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	frags := cbs[0](jpf.AgentSession{})
	if frags["working_memory"] != "the dog sat" {
		t.Fatalf("got %q", frags["working_memory"])
	}
}

func TestUpdateMemoryFailsOnNonUniqueMatch(t *testing.T) {
	tools, _ := BuildWorkingMemoryUtils()
	update := tools[0]

	if _, err := update.Call(context.Background(), jpf.ToolArgs{"old_text": "", "new_text": "a a"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := update.Call(context.Background(), jpf.ToolArgs{"old_text": "a", "new_text": "b"}); err == nil {
		t.Fatalf("expected an error for a non-unique match")
	}
}

func TestUpdateMemoryFailsWhenOldTextMissing(t *testing.T) {
	tools, _ := BuildWorkingMemoryUtils()
	update := tools[0]

	if _, err := update.Call(context.Background(), jpf.ToolArgs{"old_text": "", "new_text": "hello"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := update.Call(context.Background(), jpf.ToolArgs{"old_text": "nope", "new_text": "b"}); err == nil {
		t.Fatalf("expected an error for a missing match")
	}
}
