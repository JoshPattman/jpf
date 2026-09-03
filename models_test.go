package jpf

import (
	"image"
	"testing"
)

func TestUsageAdd(t *testing.T) {
	a := Usage{InputTokens: 1, OutputTokens: 2, SuccessfulCalls: 3, FailedCalls: 4}
	b := Usage{InputTokens: 10, OutputTokens: 20, SuccessfulCalls: 30, FailedCalls: 40}
	got := a.Add(b)
	want := Usage{InputTokens: 11, OutputTokens: 22, SuccessfulCalls: 33, FailedCalls: 44}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestModelResponseOnlyUsage(t *testing.T) {
	r := ModelResponse{Message: AssistantMessage{Content: "hi"}, Usage: Usage{InputTokens: 5}}
	got := r.OnlyUsage()
	if got.Message.Content != "" || got.Usage.InputTokens != 5 {
		t.Fatalf("got %+v", got)
	}
}

func TestModelResponseIncludingUsage(t *testing.T) {
	r := ModelResponse{Message: AssistantMessage{Content: "hi"}, Usage: Usage{InputTokens: 5}}
	got := r.IncludingUsage(Usage{InputTokens: 2})
	if got.Message.Content != "hi" || got.Usage.InputTokens != 7 {
		t.Fatalf("got %+v", got)
	}
}

func TestModelResponseOpts(t *testing.T) {
	streamer := &fakeStreamer{}
	kwargs := GetModelResponseKwargs(
		WithStreamResponse(streamer),
		WithOutputFormat("format"),
		WithToolSchemas(ToolSchema{Name: "a"}),
		WithToolSchemas(ToolSchema{Name: "b"}),
	)
	if kwargs.Streamer != streamer {
		t.Fatalf("expected streamer to be set")
	}
	if kwargs.OutputFormat != "format" {
		t.Fatalf("expected output format to be set, got %v", kwargs.OutputFormat)
	}
	if len(kwargs.ToolSchemas) != 2 || kwargs.ToolSchemas[0].Name != "a" || kwargs.ToolSchemas[1].Name != "b" {
		t.Fatalf("expected tool schemas to accumulate, got %+v", kwargs.ToolSchemas)
	}
}

func TestGetModelResponseKwargsDefault(t *testing.T) {
	kwargs := GetModelResponseKwargs()
	if kwargs.Streamer != nil || kwargs.OutputFormat != nil || len(kwargs.ToolSchemas) != 0 {
		t.Fatalf("expected zero value kwargs, got %+v", kwargs)
	}
}

type fakeStreamer struct{}

func (f *fakeStreamer) OnMessageBegin()      {}
func (f *fakeStreamer) OnMessageText(string) {}
func (f *fakeStreamer) OnMessageReset()      {}

func TestUserMessageStringAndEq(t *testing.T) {
	m := UserMessage{Content: "hello"}
	if m.String() != `UserMessage{Content: "hello", Images: 0}` {
		t.Fatalf("got %s", m.String())
	}
	if !m.Eq(UserMessage{Content: "hello"}) {
		t.Fatal("expected equal messages to be Eq")
	}
	if m.Eq(UserMessage{Content: "other"}) {
		t.Fatal("expected different content to not be Eq")
	}
	if m.Eq(SystemMessage{Content: "hello"}) {
		t.Fatal("expected different types to not be Eq")
	}
}

func TestAssistantMessageStringAndEq(t *testing.T) {
	m := AssistantMessage{Content: "hi", ToolCalls: []ToolCall{{ID: "c1", Tool: "search"}}}
	if m.String() != `AssistantMessage{Content: "hi", ToolCalls: 1}` {
		t.Fatalf("got %s", m.String())
	}
	if !m.Eq(AssistantMessage{Content: "hi", ToolCalls: []ToolCall{{ID: "c1", Tool: "search"}}}) {
		t.Fatal("expected equal messages to be Eq")
	}
	if m.Eq(AssistantMessage{Content: "hi"}) {
		t.Fatal("expected different tool calls to not be Eq")
	}
	if m.Eq(UserMessage{Content: "hi"}) {
		t.Fatal("expected different types to not be Eq")
	}
}

func TestDeveloperMessageStringAndEq(t *testing.T) {
	m := DeveloperMessage{Content: "be nice"}
	if m.String() != `DeveloperMessage{Content: "be nice"}` {
		t.Fatalf("got %s", m.String())
	}
	if !m.Eq(DeveloperMessage{Content: "be nice"}) {
		t.Fatal("expected equal messages to be Eq")
	}
	if m.Eq(DeveloperMessage{Content: "other"}) {
		t.Fatal("expected different content to not be Eq")
	}
	if m.Eq(SystemMessage{Content: "be nice"}) {
		t.Fatal("expected different types to not be Eq")
	}
}

func TestSystemMessageStringAndEq(t *testing.T) {
	m := SystemMessage{Content: "you are a bot"}
	if m.String() != `SystemMessage{Content: "you are a bot"}` {
		t.Fatalf("got %s", m.String())
	}
	if !m.Eq(SystemMessage{Content: "you are a bot"}) {
		t.Fatal("expected equal messages to be Eq")
	}
	if m.Eq(SystemMessage{Content: "other"}) {
		t.Fatal("expected different content to not be Eq")
	}
	if m.Eq(DeveloperMessage{Content: "you are a bot"}) {
		t.Fatal("expected different types to not be Eq")
	}
}

func TestToolResultMessageStringAndEq(t *testing.T) {
	m := ToolResultMessage{CallID: "c1", Result: "done"}
	if m.String() != `ToolResultMessage{CallID: "c1", Result: "done"}` {
		t.Fatalf("got %s", m.String())
	}
	if !m.Eq(ToolResultMessage{CallID: "c1", Result: "done"}) {
		t.Fatal("expected equal messages to be Eq")
	}
	if m.Eq(ToolResultMessage{CallID: "c2", Result: "done"}) {
		t.Fatal("expected different call id to not be Eq")
	}
	if m.Eq(SystemMessage{Content: "done"}) {
		t.Fatal("expected different types to not be Eq")
	}
}

func TestImageAttachmentToBase64Encoded(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, image.White)
	att := &ImageAttachment{Source: img}

	png, err := att.ToBase64Encoded(false)
	if err != nil {
		t.Fatalf("ToBase64Encoded(false): %v", err)
	}
	if !hasPrefix(png, "data:image/png;base64,") {
		t.Fatalf("expected a png data uri, got %s", png)
	}

	jpg, err := att.ToBase64Encoded(true)
	if err != nil {
		t.Fatalf("ToBase64Encoded(true): %v", err)
	}
	if !hasPrefix(jpg, "data:image/jpeg;base64,") {
		t.Fatalf("expected a jpeg data uri, got %s", jpg)
	}
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
