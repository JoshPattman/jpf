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

// recordingStreamer records every message it is given, so tests can assert on
// exactly what was streamed to the caller.
type recordingStreamer struct {
	messages []jpf.Message
}

func (r *recordingStreamer) OnMessageComplete(msg jpf.Message) {
	r.messages = append(r.messages, msg)
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
	agent := NewReAct(model)
	agent.SetToolCatalogue([]jpf.Tool{
		{
			Schema: jpf.ToolSchema{Name: "echo", Params: []jpf.ToolParam{{Name: "msg", Type: jpf.ToolParamString}}},
			Call: func(_ context.Context, m jpf.ToolArgs) (jpf.ToolResult, error) {
				return jpf.ToolResult{Content: "echoed: " + m.String("msg")}, nil
			},
		},
	})

	rec := &recordingStreamer{}
	err := agent.Run(context.Background(), "hello", jpf.WithStreamActions(rec))
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
	requireMessages(t, rec.messages, want)
}

func TestAgentRunErrorsWhenAwaitingDeferredCalls(t *testing.T) {
	agent := NewReAct(&fakeModel{})
	sess := agent.Session()
	sess.CurrentDeferredToolCalls = []jpf.DeferredToolCall{{ToolName: "fetch", CallID: "c1"}}
	agent.SetSession(sess)

	err := agent.Run(context.Background(), "hello")
	if err == nil || !strings.Contains(err.Error(), "resume") {
		t.Fatalf("expected an error mentioning resume, got: %v", err)
	}
}

func TestAgentDeferredToolCallPausesAndCanBeResumed(t *testing.T) {
	model := &fakeModel{turns: []fakeModelTurn{
		assistantTurn("", jpf.ToolCall{ID: "c1", Tool: "fetch", Args: map[string]any{"url": "http://x"}}),
	}}
	agent := NewReAct(model)
	agent.SetToolCatalogue([]jpf.Tool{
		{Schema: jpf.ToolSchema{Name: "fetch", Params: []jpf.ToolParam{{Name: "url", Type: jpf.ToolParamString}}}},
	})

	runRec := &recordingStreamer{}
	err := agent.Run(context.Background(), "go fetch", jpf.WithStreamActions(runRec))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	deferred := agent.Session().CurrentDeferredToolCalls
	if len(deferred) != 1 || deferred[0].ToolName != "fetch" || deferred[0].CallID != "c1" || deferred[0].Args["url"] != "http://x" {
		t.Fatalf("unexpected deferred calls: %+v", deferred)
	}
	// The placeholder tool result should be in history, but not yet handed to the callback.
	requireMessages(t, agent.Session().CoreMessages, []jpf.Message{
		jpf.UserMessage{Content: "go fetch"},
		jpf.AssistantMessage{ToolCalls: []jpf.ToolCall{{ID: "c1", Tool: "fetch", Args: map[string]any{"url": "http://x"}}}},
		jpf.ToolResultMessage{CallID: "c1", Result: ""},
	})
	requireMessages(t, runRec.messages, []jpf.Message{
		jpf.UserMessage{Content: "go fetch"},
		jpf.AssistantMessage{ToolCalls: []jpf.ToolCall{{ID: "c1", Tool: "fetch", Args: map[string]any{"url": "http://x"}}}},
	})

	model.turns = append(model.turns, assistantTurn("got it"))
	resumeRec := &recordingStreamer{}
	err = agent.Resume(context.Background(), []jpf.DeferredCallResult{{CallID: "c1", Content: "42"}}, jpf.WithStreamActions(resumeRec))
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if len(agent.Session().CurrentDeferredToolCalls) != 0 {
		t.Fatalf("expected no deferred calls after resume, got %+v", agent.Session().CurrentDeferredToolCalls)
	}
	requireMessages(t, agent.Session().CoreMessages, []jpf.Message{
		jpf.UserMessage{Content: "go fetch"},
		jpf.AssistantMessage{ToolCalls: []jpf.ToolCall{{ID: "c1", Tool: "fetch", Args: map[string]any{"url": "http://x"}}}},
		jpf.ToolResultMessage{CallID: "c1", Result: "42"},
		jpf.AssistantMessage{Content: "got it"},
	})
	// Resuming replays the now-resolved tool result (never shown to the callback while deferred),
	// then continues normally with the model's next message.
	requireMessages(t, resumeRec.messages, []jpf.Message{
		jpf.ToolResultMessage{CallID: "c1", Result: "42"},
		jpf.AssistantMessage{Content: "got it"},
	})
}

func TestAgentResumeErrorsWhenNotAwaitingDeferredCalls(t *testing.T) {
	agent := NewReAct(&fakeModel{})
	err := agent.Resume(context.Background(), []jpf.DeferredCallResult{{CallID: "c1", Content: "x"}})
	if err == nil || !strings.Contains(err.Error(), "run instead") {
		t.Fatalf("expected an error mentioning run instead, got: %v", err)
	}
}

func TestAgentResumeErrorsOnCallCountMismatch(t *testing.T) {
	agent := NewReAct(&fakeModel{})
	sess := agent.Session()
	sess.CurrentDeferredToolCalls = []jpf.DeferredToolCall{{ToolName: "fetch", CallID: "c1"}}
	agent.SetSession(sess)

	err := agent.Resume(context.Background(), []jpf.DeferredCallResult{{CallID: "c1", Content: "x"}, {CallID: "c2", Content: "y"}})
	if err == nil {
		t.Fatalf("expected an error for mismatched call count")
	}
}

func TestAgentResumeErrorsOnUnknownCallID(t *testing.T) {
	agent := NewReAct(&fakeModel{})
	sess := agent.Session()
	sess.CurrentDeferredToolCalls = []jpf.DeferredToolCall{{ToolName: "fetch", CallID: "c1"}}
	agent.SetSession(sess)

	err := agent.Resume(context.Background(), []jpf.DeferredCallResult{{CallID: "wrong", Content: "x"}})
	if err == nil {
		t.Fatalf("expected an error for an unrecognised call id")
	}
}

func TestAgentDeferredCallArgsAreValidatedAndCoerced(t *testing.T) {
	model := &fakeModel{turns: []fakeModelTurn{
		assistantTurn("", jpf.ToolCall{ID: "c1", Tool: "count", Args: map[string]any{"n": float64(5)}}),
	}}
	agent := NewReAct(model)
	agent.SetToolCatalogue([]jpf.Tool{
		{Schema: jpf.ToolSchema{Name: "count", Params: []jpf.ToolParam{{Name: "n", Type: jpf.ToolParamInt}}}},
	})

	if err := agent.Run(context.Background(), "count to 5"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	deferred := agent.Session().CurrentDeferredToolCalls
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
	agent := NewReAct(model)

	if err := agent.Run(context.Background(), "hi"); err != nil {
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
	agent := NewReAct(model)
	agent.SetToolCatalogue([]jpf.Tool{
		{
			Schema: jpf.ToolSchema{Name: "greet", Params: []jpf.ToolParam{{Name: "name", Type: jpf.ToolParamString}}},
			Call: func(_ context.Context, m jpf.ToolArgs) (jpf.ToolResult, error) {
				t.Fatalf("Call should not run when required args are missing")
				return jpf.ToolResult{}, nil
			},
		},
	})

	if err := agent.Run(context.Background(), "hi"); err != nil {
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
	agent := NewReAct(model)
	agent.SetMaxIterations(3)
	agent.SetToolCatalogue([]jpf.Tool{
		{
			Schema: jpf.ToolSchema{Name: "loop"},
			Call: func(_ context.Context, _ jpf.ToolArgs) (jpf.ToolResult, error) {
				return jpf.ToolResult{Content: "again"}, nil
			},
		},
	})

	// If the agent tries a 4th round trip, fakeModel panics - proving maxIterations is respected.
	if err := agent.Run(context.Background(), "go"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(model.calls) != 3 {
		t.Fatalf("expected exactly 3 model calls, got %d", len(model.calls))
	}
}

func TestAgentSessionIsCloned(t *testing.T) {
	agent := NewReAct(&fakeModel{})
	sess := agent.Session()
	sess.CoreMessages = append(sess.CoreMessages, jpf.UserMessage{Content: "leak"})

	if len(agent.Session().CoreMessages) != 0 {
		t.Fatalf("mutating a returned session leaked into the agent: %+v", agent.Session().CoreMessages)
	}
}

func TestNewAgentIncludesBuiltinTools(t *testing.T) {
	agent := NewReAct(&fakeModel{})
	names := make([]string, len(agent.(*reactAgent).toolCatalogue))
	for i, tool := range agent.(*reactAgent).toolCatalogue {
		names[i] = tool.Schema.Name
	}
	if !slices.Contains(names, "activate_skill") || !slices.Contains(names, "deactivate_skill") {
		t.Fatalf("expected builtin skill tools to be present, got %v", names)
	}

	agent.SetToolCatalogue([]jpf.Tool{{Schema: jpf.ToolSchema{Name: "custom"}}})
	names = names[:0]
	for _, tool := range agent.(*reactAgent).toolCatalogue {
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
	agent := NewReAct(model)
	agent.SetSkillCatalogue([]jpf.Skill{{Name: "golang", Description: "go help", Content: "use gofmt"}})

	if err := agent.Run(context.Background(), "help me with go"); err != nil {
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
	if err := agent.Run(context.Background(), "thanks, done"); err != nil {
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
	agent := NewReAct(model)
	agent.SetSkillCatalogue([]jpf.Skill{{Name: "kept", Description: "d", Content: "c"}})

	sess := agent.Session()
	sess.ActiveSkillNames = []string{"kept", "stale"}
	agent.SetSession(sess)

	if err := agent.Run(context.Background(), "hi"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := agent.Session().ActiveSkillNames; len(got) != 1 || got[0] != "kept" {
		t.Fatalf("expected only 'kept' to remain active, got %+v", got)
	}
}

func TestAgentIncludesSystemAndHeadStateMessages(t *testing.T) {
	model := &fakeModel{turns: []fakeModelTurn{assistantTurn("ok")}}
	agent := NewReAct(model)
	agent.SetSkillCatalogue([]jpf.Skill{{Name: "golang", Description: "when writing go code", Content: "use gofmt"}})

	if err := agent.Run(context.Background(), "hi"); err != nil {
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

func headStateContent(t *testing.T, msgs []jpf.Message) (string, bool) {
	t.Helper()
	for _, m := range msgs {
		if dev, ok := m.(jpf.DeveloperMessage); ok {
			return dev.Content, true
		}
	}
	return "", false
}

func TestAgentRendersPromptFragmentsWithoutSkillCatalogue(t *testing.T) {
	model := &fakeModel{turns: []fakeModelTurn{assistantTurn("ok")}}
	agent := NewReAct(model)

	sess := agent.Session()
	sess.PromptFragments = map[string]string{"notes": "remember to be concise"}
	agent.SetSession(sess)

	if err := agent.Run(context.Background(), "hi"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	content, ok := headStateContent(t, model.calls[0].Messages)
	if !ok {
		t.Fatalf("expected a head state message even without a skill catalogue, got %v", model.calls[0].Messages)
	}
	if !strings.Contains(content, "# Extra context") || !strings.Contains(content, "remember to be concise") {
		t.Fatalf("head state did not contain the fragment: %q", content)
	}
}

func TestAgentHasNoHeadStateWithoutSkillsOrFragments(t *testing.T) {
	model := &fakeModel{turns: []fakeModelTurn{assistantTurn("ok")}}
	agent := NewReAct(model)

	if err := agent.Run(context.Background(), "hi"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, ok := headStateContent(t, model.calls[0].Messages); ok {
		t.Fatalf("expected no head state message, got %v", model.calls[0].Messages)
	}
}

func TestAgentToolCanSetAndRemovePromptFragments(t *testing.T) {
	model := &fakeModel{turns: []fakeModelTurn{
		assistantTurn("", jpf.ToolCall{ID: "c1", Tool: "set_fragment"}),
		assistantTurn("", jpf.ToolCall{ID: "c2", Tool: "clear_fragment"}),
		assistantTurn("done"),
	}}
	agent := NewReAct(model)
	agent.SetToolCatalogue([]jpf.Tool{
		{
			Schema: jpf.ToolSchema{Name: "set_fragment"},
			Call: func(_ context.Context, _ jpf.ToolArgs) (jpf.ToolResult, error) {
				return jpf.ToolResult{
					Content:         "set",
					PromptFragments: map[string]string{"todo": "- ship it"},
				}, nil
			},
		},
		{
			Schema: jpf.ToolSchema{Name: "clear_fragment"},
			Call: func(_ context.Context, _ jpf.ToolArgs) (jpf.ToolResult, error) {
				return jpf.ToolResult{
					Content:         "cleared",
					PromptFragments: map[string]string{"todo": ""},
				}, nil
			},
		},
	})

	if err := agent.Run(context.Background(), "go"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// The fragment set by the first tool call is visible on the second model call.
	content, ok := headStateContent(t, model.calls[1].Messages)
	if !ok || !strings.Contains(content, "- ship it") {
		t.Fatalf("expected fragment to be rendered on the second model call, got %q", content)
	}

	// The second tool call clears it with an empty value, so it is gone by the third.
	if _, ok := headStateContent(t, model.calls[2].Messages); ok {
		t.Fatalf("expected no head state on the third model call after the fragment was cleared")
	}
	if _, ok := agent.Session().PromptFragments["todo"]; ok {
		t.Fatalf("expected 'todo' fragment to be removed, got %+v", agent.Session().PromptFragments)
	}
}

func TestAgentPromptFragmentsAreRenderedInKeyOrder(t *testing.T) {
	model := &fakeModel{turns: []fakeModelTurn{assistantTurn("ok")}}
	agent := NewReAct(model)

	sess := agent.Session()
	sess.PromptFragments = map[string]string{
		"zebra": "ZEBRA_VALUE",
		"alpha": "ALPHA_VALUE",
		"mike":  "MIKE_VALUE",
	}
	agent.SetSession(sess)

	if err := agent.Run(context.Background(), "hi"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	content, ok := headStateContent(t, model.calls[0].Messages)
	if !ok {
		t.Fatalf("expected a head state message")
	}
	ai, mi, zi := strings.Index(content, "ALPHA_VALUE"), strings.Index(content, "MIKE_VALUE"), strings.Index(content, "ZEBRA_VALUE")
	if !(ai < mi && mi < zi) {
		t.Fatalf("fragments not rendered in key order: alpha=%d mike=%d zebra=%d\n%s", ai, mi, zi, content)
	}
}

func TestAgentDeferredCallCanSetPromptFragment(t *testing.T) {
	model := &fakeModel{turns: []fakeModelTurn{
		assistantTurn("", jpf.ToolCall{ID: "c1", Tool: "fetch", Args: map[string]any{"url": "http://x"}}),
	}}
	agent := NewReAct(model)
	agent.SetToolCatalogue([]jpf.Tool{
		{Schema: jpf.ToolSchema{Name: "fetch", Params: []jpf.ToolParam{{Name: "url", Type: jpf.ToolParamString}}}},
	})

	if err := agent.Run(context.Background(), "go fetch"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	model.turns = append(model.turns, assistantTurn("got it"))
	err := agent.Resume(context.Background(), []jpf.DeferredCallResult{
		{CallID: "c1", Content: "42", PromptFragments: map[string]string{"last_fetch": "http://x -> 42"}},
	})
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if got := agent.Session().PromptFragments["last_fetch"]; got != "http://x -> 42" {
		t.Fatalf("expected deferred call to set the fragment, got %q", got)
	}
	content, ok := headStateContent(t, model.calls[len(model.calls)-1].Messages)
	if !ok || !strings.Contains(content, "http://x -> 42") {
		t.Fatalf("expected fragment from the deferred call to be rendered, got %q", content)
	}
}

func TestAgentHeadStatePlacementDefaultsToDeveloperBeforeConversation(t *testing.T) {
	model := &fakeModel{turns: []fakeModelTurn{assistantTurn("ok")}}
	agent := NewReAct(model)
	agent.SetSkillCatalogue([]jpf.Skill{{Name: "golang", Description: "go help", Content: "use gofmt"}})

	if err := agent.Run(context.Background(), "hi"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	sent := model.calls[0].Messages
	if len(sent) != 3 {
		t.Fatalf("expected system, head state, user; got %d: %v", len(sent), sent)
	}
	if _, ok := sent[0].(jpf.SystemMessage); !ok {
		t.Fatalf("expected first message to be a SystemMessage, got %T", sent[0])
	}
	if dev, ok := sent[1].(jpf.DeveloperMessage); !ok || !strings.Contains(dev.Content, "golang") {
		t.Fatalf("expected head state DeveloperMessage before the conversation, got %+v", sent[1])
	}
	if _, ok := sent[2].(jpf.UserMessage); !ok {
		t.Fatalf("expected the user message last, got %T", sent[2])
	}
}

func TestAgentHeadStatePlacementEmbedAfterSystem(t *testing.T) {
	model := &fakeModel{turns: []fakeModelTurn{assistantTurn("ok")}}
	agent := NewReAct(model, WithHeadStatePlacement(EmbedAfterSystem))
	agent.SetSkillCatalogue([]jpf.Skill{{Name: "golang", Description: "go help", Content: "use gofmt"}})

	if err := agent.Run(context.Background(), "hi"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	sent := model.calls[0].Messages
	if len(sent) != 2 {
		t.Fatalf("expected system and user only; got %d: %v", len(sent), sent)
	}
	sys, ok := sent[0].(jpf.SystemMessage)
	if !ok {
		t.Fatalf("expected first message to be a SystemMessage, got %T", sent[0])
	}
	if !strings.Contains(sys.Content, "# Task") || !strings.Contains(sys.Content, "golang") {
		t.Fatalf("expected head state embedded in the system prompt, got %q", sys.Content)
	}
	if _, ok := headStateContent(t, sent); ok {
		t.Fatalf("expected no standalone DeveloperMessage, got %v", sent)
	}
	if _, ok := sent[1].(jpf.UserMessage); !ok {
		t.Fatalf("expected the user message second, got %T", sent[1])
	}
}

func TestAgentHeadStatePlacementDeveloperAfterConversation(t *testing.T) {
	model := &fakeModel{turns: []fakeModelTurn{assistantTurn("ok")}}
	agent := NewReAct(model, WithHeadStatePlacement(DeveloperAfterConversation))
	agent.SetSkillCatalogue([]jpf.Skill{{Name: "golang", Description: "go help", Content: "use gofmt"}})

	if err := agent.Run(context.Background(), "hi"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	sent := model.calls[0].Messages
	if len(sent) != 3 {
		t.Fatalf("expected system, user, head state; got %d: %v", len(sent), sent)
	}
	if _, ok := sent[0].(jpf.SystemMessage); !ok {
		t.Fatalf("expected first message to be a SystemMessage, got %T", sent[0])
	}
	if _, ok := sent[1].(jpf.UserMessage); !ok {
		t.Fatalf("expected the user message before the head state, got %T", sent[1])
	}
	if dev, ok := sent[2].(jpf.DeveloperMessage); !ok || !strings.Contains(dev.Content, "golang") {
		t.Fatalf("expected head state DeveloperMessage after the conversation, got %+v", sent[2])
	}
}

func TestAgentHeadStatePlacementIsInertWhenHeadStateIsEmpty(t *testing.T) {
	for _, placement := range []HeadStatePlacement{EmbedAfterSystem, DeveloperBeforeConversation, DeveloperAfterConversation} {
		model := &fakeModel{turns: []fakeModelTurn{assistantTurn("ok")}}
		agent := NewReAct(model, WithHeadStatePlacement(placement))

		if err := agent.Run(context.Background(), "hi"); err != nil {
			t.Fatalf("placement %d: Run: %v", placement, err)
		}
		sent := model.calls[0].Messages
		if len(sent) != 2 {
			t.Fatalf("placement %d: expected system and user only, got %d: %v", placement, len(sent), sent)
		}
		if _, ok := headStateContent(t, sent); ok {
			t.Fatalf("placement %d: expected no head state message, got %v", placement, sent)
		}
	}
}

func TestRequiredArg(t *testing.T) {
	args := jpf.ToolArgs{"name": "josh"}
	if got := args.String("name"); got != "josh" {
		t.Fatalf("RequiredArg: got %q", got)
	}
}

func TestFakeModelSurfacesModelError(t *testing.T) {
	wantErr := errors.New("boom")
	model := &fakeModel{turns: []fakeModelTurn{{Err: wantErr}}}
	agent := NewReAct(model)

	err := agent.Run(context.Background(), "hi")
	if err == nil || !errors.Is(err, wantErr) {
		t.Fatalf("expected wrapped model error, got: %v", err)
	}
}
