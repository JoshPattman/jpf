package agents

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/JoshPattman/jpf"
)

// fakeModel returns one queued turn per call to Respond, and records
// exactly what it was called with so tests can assert on it.
type fakeModel struct {
	turns []fakeModelTurn
	calls []fakeModelCall
}

type fakeModelTurn struct {
	Response jpf.ModelResponse
	Err      error
}

type fakeModelCall struct {
	Messages    []jpf.Message
	ToolSchemas []jpf.ToolSchema
}

func (m *fakeModel) Respond(_ context.Context, msgs []jpf.Message, opts ...jpf.ModelResponseOpt) (jpf.ModelResponse, error) {
	kwargs := jpf.GetModelResponseKwargs(opts...)
	idx := len(m.calls)
	m.calls = append(m.calls, fakeModelCall{
		Messages:    slices.Clone(msgs),
		ToolSchemas: kwargs.ToolSchemas,
	})
	if idx >= len(m.turns) {
		panic(fmt.Sprintf("fakeModel: no turn queued for call %d", idx))
	}
	return m.turns[idx].Response, m.turns[idx].Err
}

func assistantTurn(content string, calls ...jpf.ToolCall) fakeModelTurn {
	return fakeModelTurn{Response: jpf.ModelResponse{Message: jpf.AssistantMessage{Content: content, ToolCalls: calls}}}
}

func requireMessages(t *testing.T, got, want []jpf.Message) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("message count: got %d want %d\n got: %v\nwant: %v", len(got), len(want), got, want)
	}
	for i := range want {
		if !want[i].Eq(got[i]) {
			t.Fatalf("message %d mismatch:\n got: %s\nwant: %s", i, got[i], want[i])
		}
	}
}

func TestAgentRunExecutesToolThenFinishes(t *testing.T) {
	model := &fakeModel{turns: []fakeModelTurn{
		assistantTurn("", jpf.ToolCall{ID: "c1", Tool: "echo", Args: map[string]any{"msg": "hi"}}),
		assistantTurn("done"),
	}}
	agent := NewAgent(model)
	agent.SetToolCatalogue([]Tool{
		{
			Schema: jpf.ToolSchema{Name: "echo", Args: []jpf.ToolArg{{Name: "msg", Type: jpf.ToolArgString, Required: true}}},
			Call: func(_ context.Context, m map[string]any) (ToolResult, error) {
				return ToolResult{Content: "echoed: " + RequiredArg[string](m, "msg")}, nil
			},
		},
	})

	var callback []jpf.Message
	err := agent.Run(context.Background(), "hello", func(msg jpf.Message) { callback = append(callback, msg) })
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(model.calls) != 2 {
		t.Fatalf("expected 2 model calls, got %d", len(model.calls))
	}

	want := []jpf.Message{
		jpf.UserMessage{Content: "hello"},
		jpf.AssistantMessage{ToolCalls: []jpf.ToolCall{{ID: "c1", Tool: "echo", Args: map[string]any{"msg": "hi"}}}},
		jpf.ToolResultMessage{CallID: "c1", Result: "echoed: hi"},
		jpf.AssistantMessage{Content: "done"},
	}
	requireMessages(t, agent.Session().CoreMessages, want)
	requireMessages(t, callback, want)
}

func TestAgentRunErrorsWhenAwaitingDeferredCalls(t *testing.T) {
	agent := NewAgent(&fakeModel{})
	sess := agent.Session()
	sess.CurrentDeferredToolCalls = []DeferredToolCall{{ToolName: "fetch", CallID: "c1"}}
	agent.SetSession(sess)

	err := agent.Run(context.Background(), "hello", nil)
	if err == nil || !strings.Contains(err.Error(), "resume") {
		t.Fatalf("expected an error mentioning resume, got: %v", err)
	}
}

func TestAgentDeferredToolCallPausesAndCanBeResumed(t *testing.T) {
	model := &fakeModel{turns: []fakeModelTurn{
		assistantTurn("", jpf.ToolCall{ID: "c1", Tool: "fetch", Args: map[string]any{"url": "http://x"}}),
	}}
	agent := NewAgent(model)
	agent.SetToolCatalogue([]Tool{
		{Schema: jpf.ToolSchema{Name: "fetch", Args: []jpf.ToolArg{{Name: "url", Type: jpf.ToolArgString, Required: true}}}},
	})

	var runCallback []jpf.Message
	err := agent.Run(context.Background(), "go fetch", func(msg jpf.Message) { runCallback = append(runCallback, msg) })
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	deferred := agent.CurrentDeferredToolCalls()
	if len(deferred) != 1 || deferred[0].ToolName != "fetch" || deferred[0].CallID != "c1" || deferred[0].Args["url"] != "http://x" {
		t.Fatalf("unexpected deferred calls: %+v", deferred)
	}
	// The placeholder tool result should be in history, but not yet handed to the callback.
	requireMessages(t, agent.Session().CoreMessages, []jpf.Message{
		jpf.UserMessage{Content: "go fetch"},
		jpf.AssistantMessage{ToolCalls: []jpf.ToolCall{{ID: "c1", Tool: "fetch", Args: map[string]any{"url": "http://x"}}}},
		jpf.ToolResultMessage{CallID: "c1", Result: ""},
	})
	requireMessages(t, runCallback, []jpf.Message{
		jpf.UserMessage{Content: "go fetch"},
		jpf.AssistantMessage{ToolCalls: []jpf.ToolCall{{ID: "c1", Tool: "fetch", Args: map[string]any{"url": "http://x"}}}},
	})

	model.turns = append(model.turns, assistantTurn("got it"))
	var resumeCallback []jpf.Message
	err = agent.Resume(context.Background(), []DeferredCallResponse{{CallID: "c1", Result: "42"}}, func(msg jpf.Message) { resumeCallback = append(resumeCallback, msg) })
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if len(agent.CurrentDeferredToolCalls()) != 0 {
		t.Fatalf("expected no deferred calls after resume, got %+v", agent.CurrentDeferredToolCalls())
	}
	requireMessages(t, agent.Session().CoreMessages, []jpf.Message{
		jpf.UserMessage{Content: "go fetch"},
		jpf.AssistantMessage{ToolCalls: []jpf.ToolCall{{ID: "c1", Tool: "fetch", Args: map[string]any{"url": "http://x"}}}},
		jpf.ToolResultMessage{CallID: "c1", Result: "42"},
		jpf.AssistantMessage{Content: "got it"},
	})
	// Resuming replays the now-resolved tool result (never shown to the callback while deferred),
	// then continues normally with the model's next message.
	requireMessages(t, resumeCallback, []jpf.Message{
		jpf.ToolResultMessage{CallID: "c1", Result: "42"},
		jpf.AssistantMessage{Content: "got it"},
	})
}

func TestAgentResumeErrorsWhenNotAwaitingDeferredCalls(t *testing.T) {
	agent := NewAgent(&fakeModel{})
	err := agent.Resume(context.Background(), []DeferredCallResponse{{CallID: "c1", Result: "x"}}, nil)
	if err == nil || !strings.Contains(err.Error(), "run instead") {
		t.Fatalf("expected an error mentioning run instead, got: %v", err)
	}
}

func TestAgentResumeErrorsOnCallCountMismatch(t *testing.T) {
	agent := NewAgent(&fakeModel{})
	sess := agent.Session()
	sess.CurrentDeferredToolCalls = []DeferredToolCall{{ToolName: "fetch", CallID: "c1"}}
	agent.SetSession(sess)

	err := agent.Resume(context.Background(), []DeferredCallResponse{{CallID: "c1", Result: "x"}, {CallID: "c2", Result: "y"}}, nil)
	if err == nil {
		t.Fatalf("expected an error for mismatched call count")
	}
}

func TestAgentResumeErrorsOnUnknownCallID(t *testing.T) {
	agent := NewAgent(&fakeModel{})
	sess := agent.Session()
	sess.CurrentDeferredToolCalls = []DeferredToolCall{{ToolName: "fetch", CallID: "c1"}}
	agent.SetSession(sess)

	err := agent.Resume(context.Background(), []DeferredCallResponse{{CallID: "wrong", Result: "x"}}, nil)
	if err == nil {
		t.Fatalf("expected an error for an unrecognised call id")
	}
}

func TestAgentDeferredCallArgsAreValidatedAndCoerced(t *testing.T) {
	model := &fakeModel{turns: []fakeModelTurn{
		assistantTurn("", jpf.ToolCall{ID: "c1", Tool: "count", Args: map[string]any{"n": float64(5)}}),
	}}
	agent := NewAgent(model)
	agent.SetToolCatalogue([]Tool{
		{Schema: jpf.ToolSchema{Name: "count", Args: []jpf.ToolArg{{Name: "n", Type: jpf.ToolArgInt, Required: true}}}},
	})

	if err := agent.Run(context.Background(), "count to 5", nil); err != nil {
		t.Fatalf("Run: %v", err)
	}
	deferred := agent.CurrentDeferredToolCalls()
	if len(deferred) != 1 {
		t.Fatalf("expected 1 deferred call, got %d", len(deferred))
	}
	if n, ok := deferred[0].Args["n"].(int); !ok || n != 5 {
		t.Fatalf("expected coerced int arg 5, got %#v", deferred[0].Args["n"])
	}
}

func TestAgentUnknownToolProducesErrorResult(t *testing.T) {
	model := &fakeModel{turns: []fakeModelTurn{
		assistantTurn("", jpf.ToolCall{ID: "c1", Tool: "missing_tool"}),
		assistantTurn("ok"),
	}}
	agent := NewAgent(model)

	if err := agent.Run(context.Background(), "hi", nil); err != nil {
		t.Fatalf("Run: %v", err)
	}
	msgs := agent.Session().CoreMessages
	result, ok := msgs[2].(jpf.ToolResultMessage)
	if !ok || !strings.Contains(result.Result, "could not find tool with name 'missing_tool'") {
		t.Fatalf("expected an unknown-tool error result, got: %+v", msgs[2])
	}
}

func TestAgentInvalidArgsProducesErrorResult(t *testing.T) {
	model := &fakeModel{turns: []fakeModelTurn{
		assistantTurn("", jpf.ToolCall{ID: "c1", Tool: "greet"}),
		assistantTurn("ok"),
	}}
	agent := NewAgent(model)
	agent.SetToolCatalogue([]Tool{
		{
			Schema: jpf.ToolSchema{Name: "greet", Args: []jpf.ToolArg{{Name: "name", Type: jpf.ToolArgString, Required: true}}},
			Call: func(_ context.Context, m map[string]any) (ToolResult, error) {
				t.Fatalf("Call should not run when required args are missing")
				return ToolResult{}, nil
			},
		},
	})

	if err := agent.Run(context.Background(), "hi", nil); err != nil {
		t.Fatalf("Run: %v", err)
	}
	msgs := agent.Session().CoreMessages
	result, ok := msgs[2].(jpf.ToolResultMessage)
	if !ok || !strings.Contains(result.Result, "argument 'name' is required") {
		t.Fatalf("expected a missing-arg error result, got: %+v", msgs[2])
	}
}

func TestAgentMaxIterationsStopsLoop(t *testing.T) {
	model := &fakeModel{turns: []fakeModelTurn{
		assistantTurn("", jpf.ToolCall{ID: "c1", Tool: "loop"}),
		assistantTurn("", jpf.ToolCall{ID: "c2", Tool: "loop"}),
		assistantTurn("", jpf.ToolCall{ID: "c3", Tool: "loop"}),
	}}
	agent := NewAgent(model)
	agent.SetMaxIterations(3)
	agent.SetToolCatalogue([]Tool{
		{
			Schema: jpf.ToolSchema{Name: "loop"},
			Call:   func(_ context.Context, _ map[string]any) (ToolResult, error) { return ToolResult{Content: "again"}, nil },
		},
	})

	// If the agent tries a 4th round trip, fakeModel panics - proving maxIterations is respected.
	if err := agent.Run(context.Background(), "go", nil); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(model.calls) != 3 {
		t.Fatalf("expected exactly 3 model calls, got %d", len(model.calls))
	}
}

func TestAgentSessionIsCloned(t *testing.T) {
	agent := NewAgent(&fakeModel{})
	sess := agent.Session()
	sess.CoreMessages = append(sess.CoreMessages, jpf.UserMessage{Content: "leak"})

	if len(agent.Session().CoreMessages) != 0 {
		t.Fatalf("mutating a returned session leaked into the agent: %+v", agent.Session().CoreMessages)
	}
}

func TestNewAgentIncludesBuiltinTools(t *testing.T) {
	agent := NewAgent(&fakeModel{})
	names := make([]string, len(agent.toolCatalogue))
	for i, tool := range agent.toolCatalogue {
		names[i] = tool.Schema.Name
	}
	if !slices.Contains(names, "activate_skill") || !slices.Contains(names, "deactivate_skill") {
		t.Fatalf("expected builtin skill tools to be present, got %v", names)
	}

	agent.SetToolCatalogue([]Tool{{Schema: jpf.ToolSchema{Name: "custom"}}})
	names = names[:0]
	for _, tool := range agent.toolCatalogue {
		names = append(names, tool.Schema.Name)
	}
	if !slices.Contains(names, "activate_skill") || !slices.Contains(names, "deactivate_skill") || !slices.Contains(names, "custom") {
		t.Fatalf("expected builtin tools to remain alongside custom tools, got %v", names)
	}
}

func TestAgentActivateAndDeactivateSkill(t *testing.T) {
	model := &fakeModel{turns: []fakeModelTurn{
		assistantTurn("", jpf.ToolCall{ID: "c1", Tool: "activate_skill", Args: map[string]any{"skill_name": "golang"}}),
		assistantTurn("activated"),
	}}
	agent := NewAgent(model)
	agent.SetSkillCatalogue([]Skill{{Name: "golang", Description: "go help", Content: "use gofmt"}})

	if err := agent.Run(context.Background(), "help me with go", nil); err != nil {
		t.Fatalf("Run: %v", err)
	}
	result := agent.Session().CoreMessages[2].(jpf.ToolResultMessage)
	if !strings.Contains(result.Result, "activated skill 'golang'") {
		t.Fatalf("expected activation confirmation, got: %+v", result)
	}
	if !slices.Contains(agent.Session().ActiveSkillNames, "golang") {
		t.Fatalf("expected golang skill to be active, got %+v", agent.Session().ActiveSkillNames)
	}

	model.turns = append(model.turns,
		assistantTurn("", jpf.ToolCall{ID: "c2", Tool: "deactivate_skill", Args: map[string]any{"skill_name": "golang"}}),
		assistantTurn("deactivated"),
	)
	if err := agent.Run(context.Background(), "thanks, done", nil); err != nil {
		t.Fatalf("Run: %v", err)
	}
	msgs := agent.Session().CoreMessages
	result = msgs[len(msgs)-2].(jpf.ToolResultMessage)
	if !strings.Contains(result.Result, "deactivated skill 'golang'") {
		t.Fatalf("expected deactivation confirmation, got: %+v", result)
	}
	if slices.Contains(agent.Session().ActiveSkillNames, "golang") {
		t.Fatalf("expected golang skill to no longer be active, got %+v", agent.Session().ActiveSkillNames)
	}
}

func TestAgentDeactivatesMissingActiveSkills(t *testing.T) {
	model := &fakeModel{turns: []fakeModelTurn{assistantTurn("ok")}}
	agent := NewAgent(model)
	agent.SetSkillCatalogue([]Skill{{Name: "kept", Description: "d", Content: "c"}})

	sess := agent.Session()
	sess.ActiveSkillNames = []string{"kept", "stale"}
	agent.SetSession(sess)

	if err := agent.Run(context.Background(), "hi", nil); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := agent.Session().ActiveSkillNames; len(got) != 1 || got[0] != "kept" {
		t.Fatalf("expected only 'kept' to remain active, got %+v", got)
	}
}

func TestAgentIncludesSystemAndHeadStateMessages(t *testing.T) {
	model := &fakeModel{turns: []fakeModelTurn{assistantTurn("ok")}}
	agent := NewAgent(model)
	agent.SetSkillCatalogue([]Skill{{Name: "golang", Description: "when writing go code", Content: "use gofmt"}})

	if err := agent.Run(context.Background(), "hi", nil); err != nil {
		t.Fatalf("Run: %v", err)
	}
	sent := model.calls[0].Messages
	if len(sent) != 3 {
		t.Fatalf("expected system, head state, and user messages, got %d: %v", len(sent), sent)
	}
	if _, ok := sent[0].(jpf.SystemMessage); !ok {
		t.Fatalf("expected first message to be a SystemMessage, got %T", sent[0])
	}
	head, ok := sent[1].(jpf.DeveloperMessage)
	if !ok || !strings.Contains(head.Content, "golang") {
		t.Fatalf("expected a DeveloperMessage mentioning the golang skill, got %+v", sent[1])
	}
	if _, ok := sent[2].(jpf.UserMessage); !ok {
		t.Fatalf("expected third message to be the UserMessage, got %T", sent[2])
	}
}

func TestRequiredAndOptionalArg(t *testing.T) {
	args := map[string]any{"name": "josh"}
	if got := RequiredArg[string](args, "name"); got != "josh" {
		t.Fatalf("RequiredArg: got %q", got)
	}
	if got, ok := OptionalArg[string](args, "name"); !ok || got != "josh" {
		t.Fatalf("OptionalArg present: got %q, %v", got, ok)
	}
	if got, ok := OptionalArg[string](args, "missing"); ok || got != "" {
		t.Fatalf("OptionalArg missing: got %q, %v", got, ok)
	}
}

func TestFakeModelSurfacesModelError(t *testing.T) {
	wantErr := errors.New("boom")
	model := &fakeModel{turns: []fakeModelTurn{{Err: wantErr}}}
	agent := NewAgent(model)

	err := agent.Run(context.Background(), "hi", nil)
	if err == nil || !errors.Is(err, wantErr) {
		t.Fatalf("expected wrapped model error, got: %v", err)
	}
}
