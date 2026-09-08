//go:build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/JoshPattman/jpf"
	"github.com/JoshPattman/jpf/models"
)

func TestHelloModels(t *testing.T) {
	oaiKey := os.Getenv("OPENAI_KEY")
	gemKey := os.Getenv("GEMINI_KEY")
	anthKey := os.Getenv("ANTHROPIC_KEY")
	modelsToRun := []jpf.Model{
		models.NewRemote(models.OpenAIChatCompletions, "gpt-4.1", oaiKey),
		models.NewRemote(models.OpenAIChatCompletions, "gpt-4.1", oaiKey, models.WithTemperature(0)),
		models.NewRemote(models.OpenAIChatCompletions, "gpt-4.1", oaiKey, models.WithPresencePenalty(1)),
		models.NewRemote(models.Google, "gemini-2.5-flash", gemKey),
		models.NewRemote(models.Google, "gemini-2.5-flash", gemKey, models.WithTemperature(0)),
		models.NewRemote(models.OpenAIChatCompletions, "o3-mini", oaiKey),
		models.NewRemote(models.OpenAIChatCompletions, "o3-mini", oaiKey, models.WithReasoningEffort(models.MediumReasoning)),
		models.NewRemote(models.OpenAIChatCompletions, "gpt-5", oaiKey),
		models.NewRemote(models.OpenAIChatCompletions, "gpt-5", oaiKey, models.WithVerbosity(models.MediumVerbosity)),
		models.NewRemote(models.OpenAIChatCompletions, "gpt-5.4", oaiKey, models.WithReasoningEffort(models.NoneReasoning)),
		models.NewRemote(models.OpenAIChatCompletions, "gpt-5.4", oaiKey, models.WithReasoningEffort(models.XHighReasoning)),
		models.NewRemote(models.OpenAIResponses, "gpt-4.1", oaiKey),
		models.NewRemote(models.OpenAIResponses, "gpt-4.1", oaiKey, models.WithTemperature(0)),
		models.NewRemote(models.OpenAIResponses, "gpt-5", oaiKey),
		models.NewRemote(models.OpenAIResponses, "gpt-5", oaiKey, models.WithReasoningEffort(models.MediumReasoning)),
		models.NewRemote(models.OpenAIResponses, "gpt-5", oaiKey, models.WithVerbosity(models.MediumVerbosity)),
		models.NewRemote(models.OpenAIResponses, "gpt-5.4", oaiKey, models.WithReasoningEffort(models.NoneReasoning)),
		models.NewRemote(models.OpenAIResponses, "gpt-5.4", oaiKey, models.WithReasoningEffort(models.XHighReasoning)),
		models.NewRemote(models.Anthropic, "claude-haiku-4-5", anthKey),
	}
	for i, model := range modelsToRun {
		t.Run(fmt.Sprintf("configuration_%d", i), testHelloModel(models.Timeout(model, time.Minute)))
	}
}

func testHelloModel(model jpf.Model) func(t *testing.T) {
	return func(t *testing.T) {
		resp, err := model.Respond(
			context.Background(),
			[]jpf.Message{
				jpf.UserMessage{
					Content: "Hello there!",
				},
			})
		if err != nil {
			t.Fatal(err)
		}
		if len(resp.Message.Content) == 0 {
			t.Fatal("primary message was empty")
		}
		t.Log(resp.Message.Content)
	}
}

func TestToolCallModels(t *testing.T) {
	oaiKey := os.Getenv("OPENAI_KEY")
	gemKey := os.Getenv("GEMINI_KEY")
	anthKey := os.Getenv("ANTHROPIC_KEY")
	modelsToRun := []jpf.Model{
		models.NewRemote(models.OpenAIChatCompletions, "gpt-4.1", oaiKey),
		models.NewRemote(models.Google, "gemini-2.5-flash", gemKey),
		models.NewRemote(models.OpenAIChatCompletions, "o3-mini", oaiKey),
		models.NewRemote(models.OpenAIChatCompletions, "gpt-5", oaiKey),
		models.NewRemote(models.OpenAIResponses, "gpt-4.1", oaiKey),
		models.NewRemote(models.OpenAIResponses, "gpt-5", oaiKey),
		models.NewRemote(models.Anthropic, "claude-haiku-4-5", anthKey),
	}
	for i, model := range modelsToRun {
		t.Run(fmt.Sprintf("configuration_%d", i), testToolCallModel(models.Timeout(model, time.Minute)))
	}
}

func testToolCallModel(model jpf.Model) func(t *testing.T) {
	return func(t *testing.T) {
		schemas := jpf.ToolSchema{
			Name:        "ping_user",
			Description: "ping the user, use only when asked",
			Params: []jpf.ToolParam{
				{
					Name:        "message",
					Description: "a nice message to ping the user with",
					Type:        jpf.ToolParamString,
					Required:    true,
				},
			},
		}
		msgs := []jpf.Message{
			jpf.SystemMessage{
				Content: "When calling tools, you **must** include a short natural language message explaining what you are doing. The ping tool will include a confirmation password. You **must** include that exact password in your final reasponse, as a regex will check for it.",
			},
			jpf.UserMessage{
				Content: "Ping me!",
			},
		}
		resp, err := model.Respond(context.Background(), msgs, jpf.WithToolSchemas(schemas))
		if err != nil {
			t.Fatal(err)
		}
		if len(resp.Message.ToolCalls) == 0 {
			t.Fatal("no tools were called")
		}
		if resp.Message.ToolCalls[0].Tool != "ping_user" {
			t.Fatal("wrong tool was called")
		}
		if resp.Message.Content != "" {
			t.Log(resp.Message.Content)
		}
		t.Log("AI SENT YOU A PING:", resp.Message.ToolCalls[0].Args["message"])
		msgs = append(msgs, resp.Message)
		msgs = append(msgs, jpf.ToolResultMessage{
			CallID: resp.Message.ToolCalls[0].ID,
			Result: "Ping sent. Now please make sure to include the following conformation password in your response: 'noodles'",
		})
		resp, err = model.Respond(context.Background(), msgs, jpf.WithToolSchemas(schemas))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(resp.Message.Content, "noodles") {
			t.Fatal("response did not inclide confirmation")
		}
		t.Log(resp.Message.Content)
	}
}

// TestStoreReasoningRoundTrip exercises WithStoreReasoning end to end: it makes a
// tool-calling turn, checks opaque reasoning blocks came back, then replays that
// assistant message (with its reasoning) plus a tool result and checks the
// follow-up turn succeeds - i.e. the reasoning blocks were accepted by the API.
func TestStoreReasoningRoundTrip(t *testing.T) {
	oaiKey := os.Getenv("OPENAI_KEY")
	gemKey := os.Getenv("GEMINI_KEY")
	anthKey := os.Getenv("ANTHROPIC_KEY")
	cases := []struct {
		name  string
		model jpf.Model
	}{
		{"anthropic", models.NewRemote(models.Anthropic, "claude-haiku-4-5", anthKey, models.WithStoreReasoning())},
		{"anthropic-high-effort", models.NewRemote(models.Anthropic, "claude-haiku-4-5", anthKey, models.WithStoreReasoning(), models.WithReasoningEffort(models.HighReasoning))},
		{"openai-responses", models.NewRemote(models.OpenAIResponses, "gpt-5", oaiKey, models.WithStoreReasoning(), models.WithReasoningEffort(models.LowReasoning))},
		{"gemini", models.NewRemote(models.Google, "gemini-2.5-flash", gemKey, models.WithStoreReasoning())},
		{"gemini-high-effort", models.NewRemote(models.Google, "gemini-2.5-flash", gemKey, models.WithStoreReasoning(), models.WithReasoningEffort(models.HighReasoning))},
	}
	for _, c := range cases {
		t.Run(c.name, testStoreReasoningRoundTrip(models.Timeout(c.model, time.Minute)))
	}
}

func testStoreReasoningRoundTrip(model jpf.Model) func(t *testing.T) {
	return func(t *testing.T) {
		schemas := jpf.ToolSchema{
			Name:        "ping_user",
			Description: "ping the user, use only when asked",
			Params: []jpf.ToolParam{
				{
					Name:        "message",
					Description: "a nice message to ping the user with",
					Type:        jpf.ToolParamString,
					Required:    true,
				},
			},
		}
		msgs := []jpf.Message{
			jpf.SystemMessage{Content: "When calling tools, include a short natural language message explaining what you are doing. The ping tool returns a confirmation password; you must include that exact password in your final response, as a regex will check for it."},
			jpf.UserMessage{Content: "Ping me!"},
		}
		resp, err := model.Respond(context.Background(), msgs, jpf.WithToolSchemas(schemas))
		if err != nil {
			t.Fatal(err)
		}
		if len(resp.Message.ToolCalls) == 0 {
			t.Fatal("no tools were called")
		}

		reasoningBlocks := len(resp.Message.Reasoning)
		for _, tc := range resp.Message.ToolCalls {
			reasoningBlocks += len(tc.Reasoning)
		}
		t.Logf("captured %d opaque reasoning block(s) (turn-level: %d)", reasoningBlocks, len(resp.Message.Reasoning))
		if reasoningBlocks == 0 {
			t.Fatal("expected at least one opaque reasoning block with WithStoreReasoning")
		}

		msgs = append(msgs, resp.Message)
		msgs = append(msgs, jpf.ToolResultMessage{
			CallID: resp.Message.ToolCalls[0].ID,
			Result: "Ping sent. Include this confirmation password in your response: 'noodles'",
		})
		resp, err = model.Respond(context.Background(), msgs, jpf.WithToolSchemas(schemas))
		if err != nil {
			t.Fatalf("replaying the assistant turn with its reasoning blocks failed: %v", err)
		}
		if !strings.Contains(resp.Message.Content, "noodles") {
			t.Fatalf("follow-up response did not include the confirmation: %q", resp.Message.Content)
		}
		t.Log(resp.Message.Content)
	}
}

func TestStructuredOutputs(t *testing.T) {
	oaiKey := os.Getenv("OPENAI_KEY")
	gemKey := os.Getenv("GEMINI_KEY")
	anthKey := os.Getenv("ANTHROPIC_KEY")
	modelsToRun := []jpf.Model{
		models.NewRemote(models.OpenAIChatCompletions, "gpt-4.1", oaiKey),
		models.NewRemote(models.Google, "gemini-2.5-flash", gemKey),
		models.NewRemote(models.OpenAIChatCompletions, "o3-mini", oaiKey),
		models.NewRemote(models.OpenAIChatCompletions, "gpt-5", oaiKey),
		models.NewRemote(models.OpenAIResponses, "gpt-4.1", oaiKey),
		models.NewRemote(models.OpenAIResponses, "gpt-5", oaiKey),
		models.NewRemote(models.Anthropic, "claude-haiku-4-5", anthKey),
	}
	for i, model := range modelsToRun {
		t.Run(fmt.Sprintf("configuration_%d", i), testStructuredOutput(models.Timeout(model, time.Minute)))
	}
}

type helloResponse struct {
	Sentiment string `json:"sentiment"`
	Response  string `json:"response"`
}

func testStructuredOutput(model jpf.Model) func(t *testing.T) {
	return func(t *testing.T) {
		resp, err := model.Respond(
			context.Background(),
			[]jpf.Message{
				jpf.UserMessage{
					Content: "Hello there!",
				},
			}, jpf.WithOutputFormat(helloResponse{}))
		if err != nil {
			t.Fatal(err)
		}
		if len(resp.Message.Content) == 0 {
			t.Fatal("primary message was empty")
		}
		t.Log(resp.Message.Content)
	}
}

func TestStreamToolCallModels(t *testing.T) {
	oaiKey := os.Getenv("OPENAI_KEY")
	gemKey := os.Getenv("GEMINI_KEY")
	anthKey := os.Getenv("ANTHROPIC_KEY")
	modelsToRun := []jpf.Model{
		models.NewRemote(models.OpenAIChatCompletions, "gpt-5", oaiKey),
		models.NewRemote(models.Google, "gemini-2.5-flash", gemKey),
		models.NewRemote(models.OpenAIResponses, "gpt-5", oaiKey),
		models.NewRemote(models.Anthropic, "claude-haiku-4-5", anthKey),
	}
	for i, model := range modelsToRun {
		t.Run(fmt.Sprintf("configuration_%d", i), testStreamToolCallModel(models.Timeout(model, time.Minute)))
	}
}

type streamTextCollector struct {
	text string
}

func (s *streamTextCollector) OnMessageBegin() {}
func (s *streamTextCollector) OnMessageReset() {}
func (s *streamTextCollector) OnMessageText(text string) {
	s.text += text
}

func testStreamToolCallModel(model jpf.Model) func(t *testing.T) {
	return func(t *testing.T) {
		schemas := jpf.ToolSchema{
			Name:        "ping_user",
			Description: "ping the user, use only when asked",
			Params: []jpf.ToolParam{
				{
					Name:        "message",
					Description: "a nice message to ping the user with",
					Type:        jpf.ToolParamString,
					Required:    true,
				},
			},
		}
		msgs := []jpf.Message{
			jpf.SystemMessage{
				Content: "When calling tools, you **must** include a short natural language message explaining what you are doing. The ping tool will include a confirmation password. You **must** include that exact password in your final reasponse, as a regex will check for it.",
			},
			jpf.UserMessage{
				Content: "Ping me!",
			},
		}
		streamer := &streamTextCollector{}
		resp, err := model.Respond(context.Background(), msgs, jpf.WithToolSchemas(schemas), jpf.WithStreamResponse(streamer))
		if err != nil {
			t.Fatal(err)
		}
		if len(resp.Message.ToolCalls) == 0 {
			t.Fatal("no tools were called")
		}
		if resp.Message.ToolCalls[0].Tool != "ping_user" {
			t.Fatal("wrong tool was called")
		}
		if resp.Message.Content != streamer.text {
			t.Fatal("Streamed text did not match result text")
		}
		if resp.Message.Content != "" {
			t.Log(resp.Message.Content)
		}
		t.Log("AI SENT YOU A PING:", resp.Message.ToolCalls[0].Args["message"])
		msgs = append(msgs, resp.Message)
		msgs = append(msgs, jpf.ToolResultMessage{
			CallID: resp.Message.ToolCalls[0].ID,
			Result: "Ping sent. Now please make sure to include the following conformation password in your response: 'noodles'",
		})
		streamer = &streamTextCollector{}
		resp, err = model.Respond(context.Background(), msgs, jpf.WithToolSchemas(schemas), jpf.WithStreamResponse(streamer))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(resp.Message.Content, "noodles") {
			t.Fatal("response did not inclide confirmation")
		}
		if resp.Message.Content != streamer.text {
			t.Fatal("Streamed text did not match result text")
		}
		t.Log(resp.Message.Content)
	}
}
