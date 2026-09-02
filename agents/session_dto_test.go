package agents

import (
	"encoding/json"
	"testing"

	"github.com/JoshPattman/jpf"
)

func TestAgentSessionDTORoundTrip(t *testing.T) {
	sess := AgentSession{
		AgentPrompt:       "agent",
		TaskPrompt:        "task",
		PersonalityPrompt: "personality",
		CoreMessages: []jpf.Message{
			jpf.SystemMessage{Content: "system"},
			jpf.UserMessage{Content: "hello"},
			jpf.AssistantMessage{Content: "calling", ToolCalls: []jpf.ToolCall{
				{ID: "c1", Tool: "search", Args: map[string]any{"q": "cats"}},
			}},
			jpf.ToolResultMessage{CallID: "c1", Result: "done"},
		},
		CurrentDeferredToolCalls: []DeferredToolCall{
			{CallID: "c2", Args: map[string]any{"path": "/tmp"}},
		},
		ActiveSkillNames: []string{"skill-a", "skill-b"},
	}

	var dto AgentSessionDTO
	if err := dto.LoadSession(sess); err != nil {
		t.Fatalf("LoadSession: %v", err)
	}

	data, err := json.Marshal(dto)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var loaded AgentSessionDTO
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	got, err := loaded.ToSession()
	if err != nil {
		t.Fatalf("ToSession: %v", err)
	}

	if got.AgentPrompt != sess.AgentPrompt || got.TaskPrompt != sess.TaskPrompt || got.PersonalityPrompt != sess.PersonalityPrompt {
		t.Fatalf("prompt mismatch: %+v", got)
	}
	if len(got.CoreMessages) != len(sess.CoreMessages) {
		t.Fatalf("message count: got %d want %d", len(got.CoreMessages), len(sess.CoreMessages))
	}
	for i := range sess.CoreMessages {
		if !sess.CoreMessages[i].Eq(got.CoreMessages[i]) {
			t.Fatalf("message %d mismatch:\n got: %s\nwant: %s", i, got.CoreMessages[i], sess.CoreMessages[i])
		}
	}
	if len(got.CurrentDeferredToolCalls) != 1 || got.CurrentDeferredToolCalls[0].CallID != "c2" {
		t.Fatalf("deferred calls did not round-trip: %+v", got.CurrentDeferredToolCalls)
	}
	if got.CurrentDeferredToolCalls[0].Args["path"] != "/tmp" {
		t.Fatalf("deferred call args did not round-trip: %+v", got.CurrentDeferredToolCalls[0].Args)
	}
	if len(got.ActiveSkillNames) != 2 || got.ActiveSkillNames[0] != "skill-a" || got.ActiveSkillNames[1] != "skill-b" {
		t.Fatalf("active skill names did not round-trip: %+v", got.ActiveSkillNames)
	}
}

func TestAgentSessionDTOEmptyRoundTrip(t *testing.T) {
	sess := DefaultSession()

	var dto AgentSessionDTO
	if err := dto.LoadSession(sess); err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	data, err := json.Marshal(dto)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var loaded AgentSessionDTO
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	got, err := loaded.ToSession()
	if err != nil {
		t.Fatalf("ToSession: %v", err)
	}

	if got.AgentPrompt != sess.AgentPrompt || len(got.CoreMessages) != 0 || len(got.CurrentDeferredToolCalls) != 0 {
		t.Fatalf("unexpected session: %+v", got)
	}
}

func TestAgentSessionDTOLoadSessionResetsState(t *testing.T) {
	dto := AgentSessionDTO{ActiveSkillNames: []string{"stale"}, CoreMessages: []jpf.MessageDTO{{Role: jpf.MessageRoleUser}}}
	if err := dto.LoadSession(DefaultSession()); err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	if dto.ActiveSkillNames != nil || dto.CoreMessages != nil {
		t.Fatalf("expected slices to be cleared, got %+v", dto)
	}
}
